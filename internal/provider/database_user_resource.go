package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ resource.Resource                = (*databaseUserResource)(nil)
	_ resource.ResourceWithConfigure   = (*databaseUserResource)(nil)
	_ resource.ResourceWithImportState = (*databaseUserResource)(nil)
)

// NewDatabaseUserResource returns a new laravelforge_database_user resource.
func NewDatabaseUserResource() resource.Resource {
	return &databaseUserResource{}
}

type databaseUserResource struct {
	client *client.Client
}

// databaseUserAttributes mirrors the JSON:API "attributes" of a database user
// (read shape). Only scalar fields are mapped; the create-only inputs
// (password, read_only, database_ids) are not echoed back on read.
type databaseUserAttributes struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type databaseUserResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Password     types.String `tfsdk:"password"`
	ReadOnly     types.Bool   `tfsdk:"read_only"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *databaseUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_user"
}

func (r *databaseUserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a database user on a Laravel Forge server. The only mutable attribute exposed by the API on update is the set of assigned databases (a list, not modelled in this pass), so all attributes here are create-only and changes force replacement.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the database user belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the database user.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the database user. Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for the database user. Write-only (never returned by the API) and create-only.",
				Required:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"read_only": schema.BoolAttribute{
				MarkdownDescription: "Whether the user should have read-only access to its databases. Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"status":     schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (r *databaseUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// basePath is the server-scoped collection path used for create/update/delete.
func (r *databaseUserResource) basePath(m *databaseUserResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/database/users", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *databaseUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"password": plan.Password.ValueString(),
	}
	if !plan.ReadOnly.IsNull() && !plan.ReadOnly.IsUnknown() {
		body["read_only"] = plan.ReadOnly.ValueBool()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge database user", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected database user ID", fmt.Sprintf("Forge returned a non-numeric database user ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read database user after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge database user", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only re-reads computed fields. Every configurable attribute is
// create-only (RequiresReplace); the sole API-mutable field is the assigned
// databases list, which is not modelled in this pass, so there is no scalar
// PUT body to send.
func (r *databaseUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan databaseUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read database user after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge database user", err.Error())
	}
}

// ImportState accepts "organization/server_id/database_user_id".
func (r *databaseUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/database_user_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	userID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and database_user_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), userID)...)
}

// readInto GETs the database user identified by m and fills computed fields.
// The single-resource read path is server-scoped (per the resource's
// links.self), matching the create/update/delete collection scope.
func (r *databaseUserResource) readInto(ctx context.Context, m *databaseUserResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a databaseUserAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	if m.ReadOnly.IsNull() || m.ReadOnly.IsUnknown() {
		m.ReadOnly = types.BoolValue(false)
	}
	return nil
}
