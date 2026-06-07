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
	_ resource.Resource                = (*siteNginxResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteNginxResource)(nil)
	_ resource.ResourceWithImportState = (*siteNginxResource)(nil)
)

// NewSiteNginxResource returns a new laravelforge_site_nginx resource. The
// site-level raw nginx configuration is a singleton per site: GET show + PUT
// update only, so create == update (PUT) and there is no destroy.
func NewSiteNginxResource() resource.Resource {
	return &siteNginxResource{}
}

type siteNginxResource struct {
	client *client.Client
}

// siteNginxAttributes mirrors the JSON:API "attributes" of NginxConfigResource.
// Note the read field is "content" while the write field (request body) is
// "config".
type siteNginxAttributes struct {
	Content *string `json:"content"`
}

type siteNginxResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Config       types.String `tfsdk:"config"`
}

func (r *siteNginxResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_nginx"
}

func (r *siteNginxResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the site-level raw Nginx configuration on a Laravel Forge site (singleton per site).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the site belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the site.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "Raw Nginx configuration for the site.",
				Required:            true,
			},
		},
	}
}

func (r *siteNginxResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteNginxResource) nginxPath(m *siteNginxResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/nginx", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

// write PUTs the desired config (the singleton has no create endpoint; create
// and update are both a PUT of the raw config).
func (r *siteNginxResource) write(ctx context.Context, plan *siteNginxResourceModel) error {
	body := map[string]any{"config": plan.Config.ValueString()}
	_, err := r.client.Write(ctx, "PUT", r.nginxPath(plan), body, nil)
	return err
}

func (r *siteNginxResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteNginxResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge site Nginx configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteNginxResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteNginxResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var a siteNginxAttributes
	if _, err := r.client.Get(ctx, r.nginxPath(&state), &a); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site Nginx configuration", err.Error())
		return
	}
	state.Config = types.StringPointerValue(a.Content)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteNginxResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteNginxResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge site Nginx configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: a site's raw Nginx configuration has no destroy endpoint
// (the config always exists for as long as the site does), so removing the
// resource only drops it from state.
func (r *siteNginxResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteNginxResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
