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

// --- Integration singleton resource (laravel-scheduler). ---
// POST enables (no JSON body), GET shows status, DELETE disables. There is no
// PUT, so the resource has no Update and any change recreates it.

var (
	_ resource.Resource                = (*siteLaravelSchedulerResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteLaravelSchedulerResource)(nil)
	_ resource.ResourceWithImportState = (*siteLaravelSchedulerResource)(nil)
)

// NewSiteLaravelSchedulerResource returns a new laravelforge_site_laravel_scheduler resource.
func NewSiteLaravelSchedulerResource() resource.Resource {
	return &siteLaravelSchedulerResource{}
}

type siteLaravelSchedulerResource struct {
	client *client.Client
}

// siteLaravelSchedulerAttributes mirrors the JSON:API "attributes" of the
// laravel-scheduler integration (read shape).
type siteLaravelSchedulerAttributes struct {
	Enabled          bool   `json:"enabled"`
	LaravelInstalled string `json:"laravel_installed"`
}

type siteLaravelSchedulerModel struct {
	Organization     types.String `tfsdk:"organization"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	SiteID           types.Int64  `tfsdk:"site_id"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	LaravelInstalled types.String `tfsdk:"laravel_installed"`
}

func (r *siteLaravelSchedulerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_laravel_scheduler"
}

func (r *siteLaravelSchedulerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Laravel scheduler integration on a Laravel Forge site. Enabling/disabling has no update path, so changes recreate the resource.",
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
				MarkdownDescription: "ID of the site the integration belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enabled":           schema.BoolAttribute{MarkdownDescription: "Whether the Laravel scheduler integration is enabled.", Computed: true},
			"laravel_installed": schema.StringAttribute{MarkdownDescription: "Whether Laravel is installed on the site.", Computed: true},
		},
	}
}

func (r *siteLaravelSchedulerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteLaravelSchedulerResource) path(m *siteLaravelSchedulerModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/laravel-scheduler",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteLaravelSchedulerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteLaravelSchedulerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// POST enables the integration (no request body).
	if _, err := r.client.Write(ctx, "POST", r.path(&plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Laravel scheduler integration", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Laravel scheduler integration after enable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteLaravelSchedulerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteLaravelSchedulerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Laravel scheduler integration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update never runs: every input attribute is RequiresReplace, so changes
// recreate the resource. It is required to satisfy the resource.Resource
// interface.
func (r *siteLaravelSchedulerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteLaravelSchedulerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Laravel scheduler integration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteLaravelSchedulerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteLaravelSchedulerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Laravel scheduler integration", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteLaravelSchedulerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the integration identified by m and fills the computed fields.
func (r *siteLaravelSchedulerResource) readInto(ctx context.Context, m *siteLaravelSchedulerModel) error {
	var a siteLaravelSchedulerAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Enabled = types.BoolValue(a.Enabled)
	m.LaravelInstalled = types.StringValue(a.LaravelInstalled)
	return nil
}
