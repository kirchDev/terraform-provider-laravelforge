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

// --- Singleton integration toggle: the Laravel Octane integration (one per
// site). No own id; identity is the parent ids (organization/server_id/site_id).
// Create=POST enable (req=EnableOctaneRequest: port + server), Read=GET status
// (404 -> drop from state), Delete=DELETE disable. There is no PUT, so a change
// to any input forces destroy+create (every input is RequiresReplace). ---

var (
	_ resource.Resource                = (*siteOctaneResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteOctaneResource)(nil)
	_ resource.ResourceWithImportState = (*siteOctaneResource)(nil)
)

// NewSiteOctaneResource returns a new laravelforge_site_octane resource.
func NewSiteOctaneResource() resource.Resource {
	return &siteOctaneResource{}
}

type siteOctaneResource struct {
	client *client.Client
}

// siteOctaneAttributes mirrors the JSON:API "attributes" of the Octane
// integration resource (read shape).
type siteOctaneAttributes struct {
	Enabled         string `json:"enabled"`
	OctaneInstalled bool   `json:"octane_installed"`
	Port            *int64 `json:"port"`
}

type siteOctaneResourceModel struct {
	Organization    types.String `tfsdk:"organization"`
	ServerID        types.Int64  `tfsdk:"server_id"`
	SiteID          types.Int64  `tfsdk:"site_id"`
	Port            types.String `tfsdk:"port"`
	Server          types.String `tfsdk:"server"`
	Enabled         types.String `tfsdk:"enabled"`
	OctaneInstalled types.Bool   `tfsdk:"octane_installed"`
	InstalledPort   types.Int64  `tfsdk:"installed_port"`
}

func (r *siteOctaneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_octane"
}

func (r *siteOctaneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Laravel Octane integration for a Laravel Forge site (singleton per " +
			"site). The presence of the resource means Octane is enabled; destroying it disables Octane. " +
			"The Forge API exposes no update endpoint, so changing any input forces a destroy and recreate.",
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
				MarkdownDescription: "ID of the site to enable the Octane integration on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"port": schema.StringAttribute{
				MarkdownDescription: "Port Octane listens on.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server": schema.StringAttribute{
				MarkdownDescription: "Octane application server. One of `swoole`, `roadrunner`, `frankenphp`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.StringAttribute{
				MarkdownDescription: "Whether the Octane integration is enabled, as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"octane_installed": schema.BoolAttribute{
				MarkdownDescription: "Whether the Octane package is installed on the site.",
				Computed:            true,
			},
			"installed_port": schema.Int64Attribute{
				MarkdownDescription: "Port Octane is configured to listen on, as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteOctaneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// path returns the Octane integration singleton endpoint for the site in m.
func (r *siteOctaneResource) path(m *siteOctaneResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/octane",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteOctaneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteOctaneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"port":   plan.Port.ValueString(),
		"server": plan.Server.ValueString(),
	}
	if _, err := r.client.Write(ctx, "POST", r.path(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Octane integration", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Octane integration after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteOctaneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteOctaneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Octane integration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: every configurable attribute is RequiresReplace, so any
// change forces destroy+create. It exists only to satisfy the interface.
func (r *siteOctaneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteOctaneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteOctaneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteOctaneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Octane integration", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteOctaneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the Octane integration singleton identified by m and fills the
// computed fields.
func (r *siteOctaneResource) readInto(ctx context.Context, m *siteOctaneResourceModel) error {
	var a siteOctaneAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Enabled = types.StringValue(a.Enabled)
	m.OctaneInstalled = types.BoolValue(a.OctaneInstalled)
	m.InstalledPort = types.Int64PointerValue(a.Port)
	return nil
}
