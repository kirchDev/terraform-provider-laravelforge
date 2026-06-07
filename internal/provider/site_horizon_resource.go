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

// Integration toggle singleton: the Laravel Horizon integration of a site. It
// has no own id — identity is the parent ids (organization/server_id/site_id).
// Create = POST enable (no JSON body), Read = GET status (404 -> drop from
// state), Delete = DELETE disable. There is no PUT, so any change recreates;
// the only inputs are the RequiresReplace parent ids, so Update never runs.

var (
	_ resource.Resource                = (*siteHorizonResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteHorizonResource)(nil)
	_ resource.ResourceWithImportState = (*siteHorizonResource)(nil)
)

// NewSiteHorizonResource returns a new laravelforge_site_horizon resource.
func NewSiteHorizonResource() resource.Resource {
	return &siteHorizonResource{}
}

type siteHorizonResource struct {
	client *client.Client
}

// siteHorizonAttributes mirrors the JSON:API "attributes" of the Horizon
// integration (read shape).
type siteHorizonAttributes struct {
	Enabled          string `json:"enabled"`
	HorizonInstalled bool   `json:"horizon_installed"`
}

type siteHorizonResourceModel struct {
	Organization     types.String `tfsdk:"organization"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	SiteID           types.Int64  `tfsdk:"site_id"`
	Enabled          types.String `tfsdk:"enabled"`
	HorizonInstalled types.Bool   `tfsdk:"horizon_installed"`
}

func (r *siteHorizonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_horizon"
}

func (r *siteHorizonResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Laravel Horizon integration of a Laravel Forge site (singleton per site). " +
			"The presence of the resource means Horizon is enabled; destroying it disables Horizon. There is no " +
			"update endpoint, so changing any attribute recreates the resource.",
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
				MarkdownDescription: "ID of the site to enable the Horizon integration on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enabled": schema.StringAttribute{
				MarkdownDescription: "Whether the Horizon integration is enabled.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"horizon_installed": schema.BoolAttribute{
				MarkdownDescription: "Whether Laravel Horizon is installed on the site.",
				Computed:            true,
			},
		},
	}
}

func (r *siteHorizonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// path returns the Horizon integration singleton endpoint for the site in m.
func (r *siteHorizonResource) path(m *siteHorizonResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/horizon",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteHorizonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteHorizonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Write(ctx, "POST", r.path(&plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Forge Horizon integration", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Horizon integration after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteHorizonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteHorizonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge Horizon integration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: every input attribute is RequiresReplace, so a change
// forces destroy+create. It exists only to satisfy the resource.Resource
// interface.
func (r *siteHorizonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteHorizonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteHorizonResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteHorizonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Forge Horizon integration", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteHorizonResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the Horizon integration for the site in m and fills the computed
// attributes.
func (r *siteHorizonResource) readInto(ctx context.Context, m *siteHorizonResourceModel) error {
	var a siteHorizonAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Enabled = types.StringValue(a.Enabled)
	m.HorizonInstalled = types.BoolValue(a.HorizonInstalled)
	return nil
}
