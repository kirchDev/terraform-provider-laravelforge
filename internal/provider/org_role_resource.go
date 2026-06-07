package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ resource.Resource                = (*orgRoleResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgRoleResource)(nil)
	_ resource.ResourceWithImportState = (*orgRoleResource)(nil)
)

// NewOrgRoleResource returns a new laravelforge_org_role resource.
func NewOrgRoleResource() resource.Resource {
	return &orgRoleResource{}
}

type orgRoleResource struct {
	client *client.Client
}

// orgRoleAttributes mirrors the JSON:API "attributes" of a custom role
// (CustomRoleResource). Only scalar attributes are mapped; the permissions
// relationship/array is skipped this pass.
type orgRoleAttributes struct {
	Name      string  `json:"name"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type orgRoleResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *orgRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_role"
}

func (r *orgRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom role within a Laravel Forge organization.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the role.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Role name.",
				Required:            true,
			},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *orgRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *orgRoleResource) basePath(m *orgRoleResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/roles", m.Organization.ValueString())
}

func (r *orgRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"name": plan.Name.ValueString()}

	id, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge role", err.Error())
		return
	}
	plan.ID = types.StringValue(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read role after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge role", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *orgRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"name": plan.Name.ValueString()}

	itemPath := fmt.Sprintf("%s/%s", r.basePath(&plan), plan.ID.ValueString())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge role", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read role after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%s", r.basePath(&state), state.ID.ValueString())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge role", err.Error())
	}
}

// ImportState accepts "organization/role".
func (r *orgRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/role\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// readInto GETs the role identified by m and fills the computed fields.
func (r *orgRoleResource) readInto(ctx context.Context, m *orgRoleResourceModel) error {
	itemPath := fmt.Sprintf("%s/%s", r.basePath(m), m.ID.ValueString())
	var a orgRoleAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.CreatedAt = types.StringPointerValue(a.CreatedAt)
	m.UpdatedAt = types.StringPointerValue(a.UpdatedAt)
	return nil
}
