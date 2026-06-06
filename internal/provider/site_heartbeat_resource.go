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
	_ resource.Resource                = (*siteHeartbeatResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteHeartbeatResource)(nil)
	_ resource.ResourceWithImportState = (*siteHeartbeatResource)(nil)
)

// NewSiteHeartbeatResource returns a new laravelforge_site_heartbeat resource.
func NewSiteHeartbeatResource() resource.Resource {
	return &siteHeartbeatResource{}
}

type siteHeartbeatResource struct {
	client *client.Client
}

// siteHeartbeatAttributes mirrors the JSON:API "attributes" of a heartbeat.
type siteHeartbeatAttributes struct {
	Name            string  `json:"name"`
	Status          *string `json:"status"`
	GracePeriod     int64   `json:"grace_period"`
	Frequency       int64   `json:"frequency"`
	CustomFrequency *string `json:"custom_frequency"`
	PingURL         *string `json:"ping_url"`
}

type siteHeartbeatResourceModel struct {
	Organization    types.String `tfsdk:"organization"`
	ServerID        types.Int64  `tfsdk:"server_id"`
	SiteID          types.Int64  `tfsdk:"site_id"`
	ID              types.Int64  `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Status          types.String `tfsdk:"status"`
	GracePeriod     types.Int64  `tfsdk:"grace_period"`
	Frequency       types.Int64  `tfsdk:"frequency"`
	CustomFrequency types.String `tfsdk:"custom_frequency"`
	PingURL         types.String `tfsdk:"ping_url"`
}

func (r *siteHeartbeatResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_heartbeat"
}

func (r *siteHeartbeatResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a cron-style heartbeat monitor on a Laravel Forge site.",
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
				MarkdownDescription: "ID of the site the heartbeat belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the heartbeat.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the heartbeat.",
				Required:            true,
			},
			"grace_period": schema.Int64Attribute{
				MarkdownDescription: "The duration (in minutes) after which a heartbeat is considered missing. One of 1, 2, 5, 10, 30, 60.",
				Required:            true,
			},
			"frequency": schema.Int64Attribute{
				MarkdownDescription: "The interval (in minutes) at which the client is expected to send a ping. One of 1, 5, 10, 30, 60, 1440, 10080, 312480, or -1 for a custom frequency.",
				Required:            true,
			},
			"custom_frequency": schema.StringAttribute{
				MarkdownDescription: "A cron expression representing the custom frequency at which the client is expected to send a ping, used when `frequency` is set to -1.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status":   schema.StringAttribute{MarkdownDescription: "Current heartbeat status (`pending`, `beating`, or `missing`).", Computed: true},
			"ping_url": schema.StringAttribute{MarkdownDescription: "The URL the client pings to report a heartbeat.", Computed: true},
		},
	}
}

func (r *siteHeartbeatResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteHeartbeatResource) basePath(m *siteHeartbeatResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/heartbeats", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteHeartbeatResource) writeBody(m *siteHeartbeatResourceModel) map[string]any {
	body := map[string]any{
		"name":         m.Name.ValueString(),
		"grace_period": m.GracePeriod.ValueInt64(),
		"frequency":    m.Frequency.ValueInt64(),
	}
	if !m.CustomFrequency.IsNull() && !m.CustomFrequency.IsUnknown() {
		body["custom_frequency"] = m.CustomFrequency.ValueString()
	}
	return body
}

func (r *siteHeartbeatResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteHeartbeatResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), r.writeBody(&plan), nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge heartbeat", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected heartbeat ID", fmt.Sprintf("Forge returned a non-numeric heartbeat ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read heartbeat after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteHeartbeatResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteHeartbeatResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge heartbeat", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteHeartbeatResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteHeartbeatResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, r.writeBody(&plan), nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge heartbeat", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read heartbeat after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteHeartbeatResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteHeartbeatResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge heartbeat", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id/heartbeat_id".
func (r *siteHeartbeatResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id/heartbeat_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	siteID, err2 := strconv.ParseInt(parts[2], 10, 64)
	heartbeatID, err3 := strconv.ParseInt(parts[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id, site_id and heartbeat_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), heartbeatID)...)
}

// readInto GETs the heartbeat identified by m and fills its fields.
func (r *siteHeartbeatResource) readInto(ctx context.Context, m *siteHeartbeatResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a siteHeartbeatAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Status = types.StringPointerValue(a.Status)
	m.GracePeriod = types.Int64Value(a.GracePeriod)
	m.Frequency = types.Int64Value(a.Frequency)
	m.CustomFrequency = types.StringPointerValue(a.CustomFrequency)
	m.PingURL = types.StringPointerValue(a.PingURL)
	return nil
}
