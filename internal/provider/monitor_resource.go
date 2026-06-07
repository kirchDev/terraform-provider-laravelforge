package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ resource.Resource                = (*monitorResource)(nil)
	_ resource.ResourceWithConfigure   = (*monitorResource)(nil)
	_ resource.ResourceWithImportState = (*monitorResource)(nil)
)

// NewMonitorResource returns a new laravelforge_monitor resource.
func NewMonitorResource() resource.Resource {
	return &monitorResource{}
}

type monitorResource struct {
	client *client.Client
}

type monitorResourceModel struct {
	Organization   types.String  `tfsdk:"organization"`
	ServerID       types.Int64   `tfsdk:"server_id"`
	ID             types.Int64   `tfsdk:"id"`
	Type           types.String  `tfsdk:"type"`
	Operator       types.String  `tfsdk:"operator"`
	Threshold      types.Float64 `tfsdk:"threshold"`
	Minutes        types.Int64   `tfsdk:"minutes"`
	Notify         types.String  `tfsdk:"notify"`
	Status         types.String  `tfsdk:"status"`
	State          types.String  `tfsdk:"state"`
	StateChangedAt types.String  `tfsdk:"state_changed_at"`
	CreatedAt      types.String  `tfsdk:"created_at"`
	UpdatedAt      types.String  `tfsdk:"updated_at"`
}

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a server monitor on a Laravel Forge server. Forge monitors cannot be edited in place, so every configurable attribute forces replacement. Requires the Business plan.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the monitor belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the monitor.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Metric type (`cpu_load`, `disk`, `free_memory`, or `used_memory`). Cannot be changed in place.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "Comparison operator against the threshold (`gte` or `lte`). Cannot be changed in place.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"threshold": schema.Float64Attribute{
				MarkdownDescription: "Percentage threshold to alert on once breached. Cannot be changed in place.",
				Required:            true,
				PlanModifiers:       []planmodifier.Float64{float64planmodifier.RequiresReplace()},
			},
			"notify": schema.StringAttribute{
				MarkdownDescription: "Email address notified when the monitor is in an alert state. Cannot be changed in place.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"minutes": schema.Int64Attribute{
				MarkdownDescription: "Frequency in minutes to evaluate the monitor. Cannot be changed in place.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace(), int64planmodifier.UseStateForUnknown()},
			},
			"status":           schema.StringAttribute{MarkdownDescription: "Installation status of the monitor (e.g. `installed`).", Computed: true},
			"state":            schema.StringAttribute{MarkdownDescription: "Current state of the monitor (`OK`, `ALERT`, or `UNKNOWN`).", Computed: true},
			"state_changed_at": schema.StringAttribute{MarkdownDescription: "Timestamp the monitor state last changed.", Computed: true},
			"created_at":       schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":       schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *monitorResource) basePath(m *monitorResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/monitors", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"type":      plan.Type.ValueString(),
		"operator":  plan.Operator.ValueString(),
		"threshold": plan.Threshold.ValueFloat64(),
		"notify":    plan.Notify.ValueString(),
	}
	if !plan.Minutes.IsNull() && !plan.Minutes.IsUnknown() {
		body["minutes"] = plan.Minutes.ValueInt64()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge monitor", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected monitor ID", fmt.Sprintf("Forge returned a non-numeric monitor ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read monitor after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge monitor", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is effectively unreachable: every configurable attribute is
// RequiresReplace and Forge exposes no monitor update endpoint (no
// server:update-monitors scope). It re-reads to keep computed fields fresh.
func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge monitor", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge monitor", err.Error())
	}
}

// ImportState accepts "organization/server_id/monitor_id".
func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/monitor_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	monitorID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and monitor_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), monitorID)...)
}

// readInto GETs the monitor identified by m and fills computed/optional fields.
// Single-monitor reads are server-scoped (per the resource's links.self).
func (r *monitorResource) readInto(ctx context.Context, m *monitorResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a monitorAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Type = types.StringValue(a.Type)
	m.Operator = types.StringValue(a.Operator)
	m.Threshold = types.Float64Value(a.Threshold)
	m.Minutes = types.Int64PointerValue(a.Minutes)
	m.Notify = types.StringValue(a.Notify)
	m.Status = types.StringValue(a.Status)
	m.State = types.StringValue(a.State)
	m.StateChangedAt = types.StringPointerValue(a.StateChangedAt)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}
