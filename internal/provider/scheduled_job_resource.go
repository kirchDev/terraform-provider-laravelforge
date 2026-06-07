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
	_ resource.Resource                = (*scheduledJobResource)(nil)
	_ resource.ResourceWithConfigure   = (*scheduledJobResource)(nil)
	_ resource.ResourceWithImportState = (*scheduledJobResource)(nil)
)

// NewScheduledJobResource returns a new laravelforge_scheduled_job resource.
func NewScheduledJobResource() resource.Resource {
	return &scheduledJobResource{}
}

type scheduledJobResource struct {
	client *client.Client
}

type scheduledJobResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Command      types.String `tfsdk:"command"`
	User         types.String `tfsdk:"user"`
	Frequency    types.String `tfsdk:"frequency"`
	Cron         types.String `tfsdk:"cron"`
	Status       types.String `tfsdk:"status"`
	NextRunTime  types.String `tfsdk:"next_run_time"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *scheduledJobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scheduled_job"
}

func (r *scheduledJobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The Forge API has no update endpoint for scheduled jobs (the item route
	// supports only GET, HEAD, DELETE — verified 2026-06-06), so every
	// configurable attribute forces replacement.
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a scheduled job (cron job) on a Laravel Forge server. Scheduled jobs are immutable: the Forge API exposes no update endpoint, so any change to a configured attribute replaces the job.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to create the scheduled job on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the scheduled job.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"command": schema.StringAttribute{
				MarkdownDescription: "Command the job executes.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "System user the command runs as (e.g. `root`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"frequency": schema.StringAttribute{
				MarkdownDescription: "How often the job runs. Sent lowercase: one of `minutely`, `hourly`, `nightly`, `weekly`, `monthly`, `reboot`, `custom`. When `custom`, also set `cron`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Optional job name.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cron": schema.StringAttribute{
				MarkdownDescription: "Cron expression. Required when `frequency` is `custom`; for preset frequencies Forge generates it.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"status":        schema.StringAttribute{MarkdownDescription: "Provisioning status (e.g. `installing`, `installed`).", Computed: true},
			"next_run_time": schema.StringAttribute{MarkdownDescription: "Timestamp of the next scheduled run.", Computed: true},
			"created_at":    schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":    schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (r *scheduledJobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *scheduledJobResource) basePath(m *scheduledJobResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/scheduled-jobs", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *scheduledJobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scheduledJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"command":   plan.Command.ValueString(),
		"user":      plan.User.ValueString(),
		"frequency": plan.Frequency.ValueString(),
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.Cron.IsNull() && !plan.Cron.IsUnknown() {
		body["cron"] = plan.Cron.ValueString()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge scheduled job", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected scheduled job ID", fmt.Sprintf("Forge returned a non-numeric scheduled job ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read scheduled job after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *scheduledJobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scheduledJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge scheduled job", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run for a real attribute change because every configurable
// attribute is RequiresReplace and the Forge API has no update endpoint. It is
// implemented only to satisfy the resource.Resource interface; it persists the
// plan and re-reads the server-owned computed fields.
func (r *scheduledJobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scheduledJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read scheduled job after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *scheduledJobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scheduledJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge scheduled job", err.Error())
	}
}

// ImportState accepts "organization/server_id/scheduled_job_id".
func (r *scheduledJobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/scheduled_job_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	jobID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and scheduled_job_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), jobID)...)
}

// readInto GETs the scheduled job identified by m and fills the computed fields.
// Single-job reads are server-scoped (per the resource's links.self).
//
// The configured RequiresReplace inputs (command, user, frequency, name) are
// intentionally NOT refreshed from the response: the API echoes "frequency"
// capitalized (e.g. "Custom") while the config sends it lowercase ("custom"),
// so overwriting state from the read value would cause perpetual drift.
func (r *scheduledJobResource) readInto(ctx context.Context, m *scheduledJobResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a scheduledJobAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Cron = types.StringPointerValue(a.Cron)
	m.Status = types.StringValue(a.Status)
	m.NextRunTime = types.StringPointerValue(a.NextRunTime)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}
