package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ resource.Resource                = (*siteDeployKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteDeployKeyResource)(nil)
	_ resource.ResourceWithImportState = (*siteDeployKeyResource)(nil)
)

// NewSiteDeployKeyResource returns a new laravelforge_site_deploy_key resource.
func NewSiteDeployKeyResource() resource.Resource {
	return &siteDeployKeyResource{}
}

type siteDeployKeyResource struct {
	client *client.Client
}

// siteDeployKeyAttributes mirrors the JSON:API "attributes" of a deploy key.
type siteDeployKeyAttributes struct {
	Key *string `json:"key"`
}

// siteDeployKeyModel is the state shape. The deploy key is a singleton per
// site (no own id) — identity is the parent ids only.
type siteDeployKeyModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Key          types.String `tfsdk:"key"`
}

func (r *siteDeployKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deploy_key"
}

func (r *siteDeployKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the deploy key for a Laravel Forge site. Singleton per site: one deploy key per site, created on POST and removed on DELETE. There is no update — any change recreates it.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server that owns the site.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the site the deploy key belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The public deploy key for the site.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteDeployKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteDeployKeyResource) path(m *siteDeployKeyModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deploy-key", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteDeployKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteDeployKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// POST creates the deploy key; no request body. The response is the
	// DeployKeyResource, but provisioning is async so the key may be absent
	// until a follow-up read — read it back to populate state.
	if _, err := r.client.Write(ctx, "POST", r.path(&plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge site deploy key", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read deploy key after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDeployKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteDeployKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site deploy key", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can't happen: every input attribute is RequiresReplace and "key" is
// computed, so changes recreate the resource. Defined to satisfy the interface.
func (r *siteDeployKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteDeployKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site deploy key", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDeployKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteDeployKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge site deploy key", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteDeployKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	siteID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and site_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
}

// readInto GETs the site's deploy key and fills the computed "key" field.
func (r *siteDeployKeyResource) readInto(ctx context.Context, m *siteDeployKeyModel) error {
	var a siteDeployKeyAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Key = types.StringPointerValue(a.Key)
	return nil
}
