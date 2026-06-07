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

// --- Singleton integration toggle. The Laravel Pulse integration has no own
// id and no PUT: POST enables it (no JSON body), GET shows its status, and
// DELETE disables it. The presence of the resource in state means enabled.
// Every identity attribute is RequiresReplace, so any change forces
// destroy+create (there is no update endpoint). A matching data source exists
// because the GET (show) endpoint does. ---

var (
	_ resource.Resource                = (*sitePulseResource)(nil)
	_ resource.ResourceWithConfigure   = (*sitePulseResource)(nil)
	_ resource.ResourceWithImportState = (*sitePulseResource)(nil)
)

// NewSitePulseResource returns a new laravelforge_site_pulse resource.
func NewSitePulseResource() resource.Resource {
	return &sitePulseResource{}
}

type sitePulseResource struct {
	client *client.Client
}

// sitePulseAttributes mirrors the JSON:API "attributes" of the Pulse
// integration (read shape). Reused by the data source.
type sitePulseAttributes struct {
	Enabled        string `json:"enabled"`
	PulseInstalled bool   `json:"pulse_installed"`
}

type sitePulseResourceModel struct {
	Organization   types.String `tfsdk:"organization"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	SiteID         types.Int64  `tfsdk:"site_id"`
	Enabled        types.String `tfsdk:"enabled"`
	PulseInstalled types.Bool   `tfsdk:"pulse_installed"`
}

func (r *sitePulseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_pulse"
}

func (r *sitePulseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables the Laravel Pulse integration on a Laravel Forge site. This is a " +
			"singleton toggle: the presence of the resource means the integration is enabled, and " +
			"destroying it disables the integration. There is no update endpoint, so any change forces " +
			"a destroy+create.",
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
				MarkdownDescription: "ID of the site to enable the Pulse integration on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enabled":         schema.StringAttribute{MarkdownDescription: "Whether the Pulse integration is enabled.", Computed: true},
			"pulse_installed": schema.BoolAttribute{MarkdownDescription: "Whether Laravel Pulse is installed on the site.", Computed: true},
		},
	}
}

func (r *sitePulseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// path returns the Pulse integration singleton endpoint for the site in m.
func (r *sitePulseResource) path(m *sitePulseResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/pulse",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *sitePulseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sitePulseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Write(ctx, "POST", r.path(&plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Pulse integration", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Pulse integration after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sitePulseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sitePulseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Pulse integration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: every attribute is RequiresReplace, so a change forces
// destroy+create. It exists only to satisfy the resource.Resource interface.
func (r *sitePulseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sitePulseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sitePulseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sitePulseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Pulse integration", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *sitePulseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the Pulse integration status for the site in m and fills its
// computed fields.
func (r *sitePulseResource) readInto(ctx context.Context, m *sitePulseResourceModel) error {
	var a sitePulseAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Enabled = types.StringValue(a.Enabled)
	m.PulseInstalled = types.BoolValue(a.PulseInstalled)
	return nil
}
