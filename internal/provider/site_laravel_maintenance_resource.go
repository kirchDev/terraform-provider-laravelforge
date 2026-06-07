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

// --- Singleton integration toggle. laravel-maintenance (Laravel maintenance
// mode) is one-per-site: POST enables (req=EnableMaintenanceModeRequest), GET
// shows status, DELETE disables. The API exposes no PUT, so every writable
// change forces destroy+create (all writable inputs are RequiresReplace). The
// resource has no own id; its identity is the parent organization/server/site. ---

var (
	_ resource.Resource                = (*siteLaravelMaintenanceResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteLaravelMaintenanceResource)(nil)
	_ resource.ResourceWithImportState = (*siteLaravelMaintenanceResource)(nil)
)

// NewSiteLaravelMaintenanceResource returns a new laravelforge_site_laravel_maintenance resource.
func NewSiteLaravelMaintenanceResource() resource.Resource {
	return &siteLaravelMaintenanceResource{}
}

type siteLaravelMaintenanceResource struct {
	client *client.Client
}

// siteLaravelMaintenanceAttributes mirrors the JSON:API "attributes" of the
// laravel-maintenance integration (read shape). Note the response "status" is a
// string enum (enabling/disabling), distinct from the request "status" HTTP
// code which is exposed as the status_code attribute.
type siteLaravelMaintenanceAttributes struct {
	Enabled          bool    `json:"enabled"`
	Status           *string `json:"status"`
	LaravelInstalled bool    `json:"laravel_installed"`
}

type siteLaravelMaintenanceResourceModel struct {
	Organization     types.String `tfsdk:"organization"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	SiteID           types.Int64  `tfsdk:"site_id"`
	StatusCode       types.Int64  `tfsdk:"status_code"`
	Secret           types.String `tfsdk:"secret"`
	Redirect         types.String `tfsdk:"redirect"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Status           types.String `tfsdk:"status"`
	LaravelInstalled types.Bool   `tfsdk:"laravel_installed"`
}

func (r *siteLaravelMaintenanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_laravel_maintenance"
}

func (r *siteLaravelMaintenanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables Laravel maintenance mode for a Laravel Forge site. This is a " +
			"singleton integration: the presence of the resource means maintenance mode is enabled, " +
			"and destroying it disables it. The Forge API exposes no update endpoint, so changing any " +
			"writable input recreates the resource.",
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
				MarkdownDescription: "ID of the site to enable maintenance mode on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"status_code": schema.Int64Attribute{
				MarkdownDescription: "The HTTP status code returned while in maintenance mode. One of 304, 307, 410, 503.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "The secret phrase that allows access to the application while in maintenance mode.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"redirect": schema.StringAttribute{
				MarkdownDescription: "The redirect URL to which all requests are sent while in maintenance mode.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled":           schema.BoolAttribute{MarkdownDescription: "Whether the maintenance mode integration is enabled.", Computed: true},
			"status":            schema.StringAttribute{MarkdownDescription: "The status of the maintenance mode integration (`enabling` or `disabling`).", Computed: true},
			"laravel_installed": schema.BoolAttribute{MarkdownDescription: "Whether Laravel is installed on the site.", Computed: true},
		},
	}
}

func (r *siteLaravelMaintenanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// path returns the laravel-maintenance singleton endpoint for the site in m.
func (r *siteLaravelMaintenanceResource) path(m *siteLaravelMaintenanceResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/laravel-maintenance",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteLaravelMaintenanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteLaravelMaintenanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"status": plan.StatusCode.ValueInt64()}
	if !plan.Secret.IsNull() && !plan.Secret.IsUnknown() {
		body["secret"] = plan.Secret.ValueString()
	}
	if !plan.Redirect.IsNull() && !plan.Redirect.IsUnknown() {
		body["redirect"] = plan.Redirect.ValueString()
	}

	if _, err := r.client.Write(ctx, "POST", r.path(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Laravel maintenance mode", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Laravel maintenance mode after enable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteLaravelMaintenanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteLaravelMaintenanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Laravel maintenance mode", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: every writable attribute is RequiresReplace (the API has
// no update endpoint), so a change forces destroy+create. It exists only to
// satisfy the resource.Resource interface.
func (r *siteLaravelMaintenanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteLaravelMaintenanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteLaravelMaintenanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteLaravelMaintenanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Laravel maintenance mode", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteLaravelMaintenanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the laravel-maintenance integration for the site in m and fills
// its computed fields.
func (r *siteLaravelMaintenanceResource) readInto(ctx context.Context, m *siteLaravelMaintenanceResourceModel) error {
	var a siteLaravelMaintenanceAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Enabled = types.BoolValue(a.Enabled)
	m.Status = types.StringPointerValue(a.Status)
	m.LaravelInstalled = types.BoolValue(a.LaravelInstalled)
	return nil
}
