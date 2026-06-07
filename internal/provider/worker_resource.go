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

// --- CRUD resource for the Forge worker (server background process). ---
//
// In the new org-scoped Forge API the legacy site-scoped "queue worker" has
// become the server-scoped "background process" (tag "Background Processes",
// type "backgroundProcesses"). Verified against the live API (2026-06-06):
//   - list/create:  GET|POST /api/orgs/{org}/servers/{server}/background-processes
//   - read/update/delete: GET|PUT|DELETE .../background-processes/{id}
//     (item path Allow header: GET, HEAD, PUT, DELETE — so update is PUT)
//   - read response is server-scoped (no org-level path; org-level returns 404)
//   - create body is FLAT; read attributes are command/user/directory/processes/
//     status/created_at. The create-only inputs name/site_id/startsecs/
//     stopwaitsecs/stopsignal are NOT echoed back on read.

var (
	_ resource.Resource                = (*workerResource)(nil)
	_ resource.ResourceWithConfigure   = (*workerResource)(nil)
	_ resource.ResourceWithImportState = (*workerResource)(nil)
)

// NewWorkerResource returns a new laravelforge_worker resource.
func NewWorkerResource() resource.Resource {
	return &workerResource{}
}

type workerResource struct {
	client *client.Client
}

type workerResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Command      types.String `tfsdk:"command"`
	User         types.String `tfsdk:"user"`
	Directory    types.String `tfsdk:"directory"`
	Processes    types.Int64  `tfsdk:"processes"`
	Startsecs    types.Int64  `tfsdk:"startsecs"`
	Stopwaitsecs types.Int64  `tfsdk:"stopwaitsecs"`
	Stopsignal   types.String `tfsdk:"stopsignal"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *workerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker"
}

func (r *workerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a worker (server background process) on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to create the worker on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the worker (background process).",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the worker. Required on create; updatable.",
				Required:            true,
			},
			"command": schema.StringAttribute{
				MarkdownDescription: "Command the worker runs.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "System user the worker runs as.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"processes": schema.Int64Attribute{
				MarkdownDescription: "Number of processes the worker runs.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "Optional ID of the site to associate the worker with. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"directory": schema.StringAttribute{
				MarkdownDescription: "Working directory of the worker.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"startsecs": schema.Int64Attribute{
				MarkdownDescription: "Seconds the process must stay up to be considered started. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"stopwaitsecs": schema.Int64Attribute{
				MarkdownDescription: "Seconds to wait for a graceful stop before killing the process. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"stopsignal": schema.StringAttribute{
				MarkdownDescription: "Signal used to stop the process (e.g. `SIGTERM`). Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status":     schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *workerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workerResource) basePath(m *workerResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/background-processes", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *workerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":      plan.Name.ValueString(),
		"command":   plan.Command.ValueString(),
		"user":      plan.User.ValueString(),
		"processes": plan.Processes.ValueInt64(),
	}
	if !plan.SiteID.IsNull() && !plan.SiteID.IsUnknown() {
		body["site_id"] = plan.SiteID.ValueInt64()
	}
	if !plan.Directory.IsNull() && !plan.Directory.IsUnknown() {
		body["directory"] = plan.Directory.ValueString()
	}
	if !plan.Startsecs.IsNull() && !plan.Startsecs.IsUnknown() {
		body["startsecs"] = plan.Startsecs.ValueInt64()
	}
	if !plan.Stopwaitsecs.IsNull() && !plan.Stopwaitsecs.IsUnknown() {
		body["stopwaitsecs"] = plan.Stopwaitsecs.ValueInt64()
	}
	if !plan.Stopsignal.IsNull() && !plan.Stopsignal.IsUnknown() {
		body["stopsignal"] = plan.Stopsignal.ValueString()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge worker", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected worker ID", fmt.Sprintf("Forge returned a non-numeric worker ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read worker after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge worker", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only "name" is updatable in place (PUT); all other fields are
	// RequiresReplace, so the plan's name is the only thing to push.
	body := map[string]any{"name": plan.Name.ValueString()}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge worker", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read worker after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge worker", err.Error())
	}
}

// ImportState accepts "organization/server_id/worker_id".
func (r *workerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/worker_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	workerID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and worker_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), workerID)...)
}

// readInto GETs the worker identified by m and fills the computed fields. Reads
// are server-scoped (the create/list path; no org-level path exists). Only the
// scalars echoed by the read endpoint (command/user/directory/processes/status/
// created_at) are refreshed; write-only inputs are left as-is in state.
func (r *workerResource) readInto(ctx context.Context, m *workerResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a workerAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Command = types.StringValue(a.Command)
	m.User = types.StringValue(a.User)
	m.Directory = types.StringPointerValue(a.Directory)
	m.Processes = types.Int64Value(a.Processes)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	return nil
}
