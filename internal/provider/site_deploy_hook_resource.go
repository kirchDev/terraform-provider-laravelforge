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

// --- Singleton-resource pattern: the deploy-hook URL/token for a site. ---
// There is no own resource id; identity is the parent ids (organization,
// server_id, site_id). Read = GET show; Create/Update = PUT the singleton
// path (no JSON body — PUT regenerates/sets the hook). There is no destroy
// endpoint, so Delete is a no-op that just drops the resource from state.

var (
	_ resource.Resource                = (*siteDeployHookResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteDeployHookResource)(nil)
	_ resource.ResourceWithImportState = (*siteDeployHookResource)(nil)
)

// NewSiteDeployHookResource returns a new laravelforge_site_deploy_hook resource.
func NewSiteDeployHookResource() resource.Resource {
	return &siteDeployHookResource{}
}

type siteDeployHookResource struct {
	client *client.Client
}

// siteDeployHookAttributes mirrors the JSON:API "attributes" of the
// DeployHookResource. Defined once and reused by the data source.
type siteDeployHookAttributes struct {
	URL string `json:"url"`
}

type siteDeployHookResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	URL          types.String `tfsdk:"url"`
}

func (r *siteDeployHookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deploy_hook"
}

func (r *siteDeployHookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the deployment-trigger (deploy-hook) URL for a Laravel Forge site. Singleton per site; applying it (re)generates the hook token.",
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
				MarkdownDescription: "ID of the site whose deploy hook is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The deployment-trigger URL for the site.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteDeployHookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteDeployHookResource) hookPath(m *siteDeployHookResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deployments/deploy-hook",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteDeployHookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteDeployHookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PUT has no JSON body; it sets/regenerates the hook and returns the URL.
	var a siteDeployHookAttributes
	if _, err := r.client.Write(ctx, "PUT", r.hookPath(&plan), nil, &a); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge site deploy hook", err.Error())
		return
	}
	plan.URL = types.StringValue(a.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDeployHookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteDeployHookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var a siteDeployHookAttributes
	if _, err := r.client.Get(ctx, r.hookPath(&state), &a); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site deploy hook", err.Error())
		return
	}
	state.URL = types.StringValue(a.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteDeployHookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteDeployHookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var a siteDeployHookAttributes
	if _, err := r.client.Write(ctx, "PUT", r.hookPath(&plan), nil, &a); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge site deploy hook", err.Error())
		return
	}
	plan.URL = types.StringValue(a.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: Forge exposes no endpoint to remove the deploy hook, so
// the resource is simply dropped from state.
func (r *siteDeployHookResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteDeployHookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
