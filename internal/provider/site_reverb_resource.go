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

// --- Singleton integration-toggle resource (no own id). ---
//
// The Reverb integration is a singleton per site: there is exactly one (or
// none). Its identity is the parent path (organization/server_id/site_id).
// POST enables it (EnableReverbRequest), GET shows status, DELETE disables it.
// There is no update endpoint, so any change to an input recreates it.

var (
	_ resource.Resource                = (*siteReverbResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteReverbResource)(nil)
	_ resource.ResourceWithImportState = (*siteReverbResource)(nil)
)

// NewSiteReverbResource returns a new laravelforge_site_reverb resource.
func NewSiteReverbResource() resource.Resource {
	return &siteReverbResource{}
}

type siteReverbResource struct {
	client *client.Client
}

// siteReverbAttributes mirrors the JSON:API "attributes" of the Reverb
// integration (read shape). Note: the create request takes `port` as a string,
// but the show response returns it as an integer.
type siteReverbAttributes struct {
	Enabled         string  `json:"enabled"`
	ReverbInstalled bool    `json:"reverb_installed"`
	Host            *string `json:"host"`
	Port            *int64  `json:"port"`
	Connections     *int64  `json:"connections"`
}

type siteReverbResourceModel struct {
	Organization         types.String `tfsdk:"organization"`
	ServerID             types.Int64  `tfsdk:"server_id"`
	SiteID               types.Int64  `tfsdk:"site_id"`
	Host                 types.String `tfsdk:"host"`
	Port                 types.String `tfsdk:"port"`
	Connections          types.Int64  `tfsdk:"connections"`
	Enabled              types.String `tfsdk:"enabled"`
	ReverbInstalled      types.Bool   `tfsdk:"reverb_installed"`
	InstalledHost        types.String `tfsdk:"installed_host"`
	InstalledPort        types.Int64  `tfsdk:"installed_port"`
	InstalledConnections types.Int64  `tfsdk:"installed_connections"`
}

func (r *siteReverbResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_reverb"
}

func (r *siteReverbResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Laravel Reverb integration on a Forge site. The integration is a singleton per site; there is no update endpoint, so changing any input recreates it.",
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
				MarkdownDescription: "ID of the site to enable Reverb on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Reverb host. Changing it recreates the integration.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"port": schema.StringAttribute{
				MarkdownDescription: "Reverb port. Changing it recreates the integration.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connections": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of concurrent connections (1-50000). Changing it recreates the integration.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enabled": schema.StringAttribute{
				MarkdownDescription: "Whether the Reverb integration is enabled, as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"reverb_installed": schema.BoolAttribute{
				MarkdownDescription: "Whether the Reverb package is installed on the site.",
				Computed:            true,
			},
			"installed_host": schema.StringAttribute{
				MarkdownDescription: "Reverb host as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"installed_port": schema.Int64Attribute{
				MarkdownDescription: "Reverb port as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"installed_connections": schema.Int64Attribute{
				MarkdownDescription: "Maximum concurrent connections as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteReverbResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteReverbResource) path(m *siteReverbResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/reverb",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteReverbResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteReverbResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"host":        plan.Host.ValueString(),
		"port":        plan.Port.ValueString(),
		"connections": plan.Connections.ValueInt64(),
	}
	if _, err := r.client.Write(ctx, "POST", r.path(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Forge Reverb integration", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Reverb integration after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteReverbResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteReverbResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge Reverb integration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: every configurable attribute is RequiresReplace and the
// Forge API exposes no update endpoint, so any change forces destroy+create. It
// exists only to satisfy the interface.
func (r *siteReverbResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteReverbResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteReverbResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteReverbResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Forge Reverb integration", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteReverbResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the Reverb integration singleton identified by m and fills the
// computed read-back fields. The configured inputs (host/port/connections) are
// left untouched — they are RequiresReplace and the response reports `port` as
// an integer, so the read-back lands in the separate installed_* attributes.
func (r *siteReverbResource) readInto(ctx context.Context, m *siteReverbResourceModel) error {
	var a siteReverbAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Enabled = types.StringValue(a.Enabled)
	m.ReverbInstalled = types.BoolValue(a.ReverbInstalled)
	m.InstalledHost = types.StringPointerValue(a.Host)
	m.InstalledPort = types.Int64PointerValue(a.Port)
	m.InstalledConnections = types.Int64PointerValue(a.Connections)
	return nil
}
