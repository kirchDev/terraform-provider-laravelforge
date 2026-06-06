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
	_ resource.Resource                = (*serverBackgroundProcessResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverBackgroundProcessResource)(nil)
	_ resource.ResourceWithImportState = (*serverBackgroundProcessResource)(nil)
)

// NewServerBackgroundProcessResource returns a new
// laravelforge_server_background_process resource.
func NewServerBackgroundProcessResource() resource.Resource {
	return &serverBackgroundProcessResource{}
}

type serverBackgroundProcessResource struct {
	client *client.Client
}

// serverBackgroundProcessAttributes mirrors the JSON:API "attributes" of a
// background process (read shape). Note the create-only inputs (name, site_id,
// startsecs, stopwaitsecs, stopsignal) and update-only input (config) are not
// returned by the read endpoint.
type serverBackgroundProcessAttributes struct {
	Command   string  `json:"command"`
	User      string  `json:"user"`
	Directory *string `json:"directory"`
	Processes int64   `json:"processes"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type serverBackgroundProcessResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Type         types.String `tfsdk:"type"`
	Name         types.String `tfsdk:"name"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Command      types.String `tfsdk:"command"`
	User         types.String `tfsdk:"user"`
	Directory    types.String `tfsdk:"directory"`
	Processes    types.Int64  `tfsdk:"processes"`
	StartSecs    types.Int64  `tfsdk:"startsecs"`
	StopWaitSecs types.Int64  `tfsdk:"stopwaitsecs"`
	StopSignal   types.String `tfsdk:"stopsignal"`
	Config       types.String `tfsdk:"config"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *serverBackgroundProcessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_background_process"
}

func (r *serverBackgroundProcessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a background process (long-running daemon-like process) on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the background process runs on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the background process.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "JSON:API resource type.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the background process.",
				Required:            true,
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "The site to associate the background process with. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"command": schema.StringAttribute{
				MarkdownDescription: "The command to run. Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "The user to run the background process as (`root` or `forge`). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"directory": schema.StringAttribute{
				MarkdownDescription: "The directory to run the background process from. Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"processes": schema.Int64Attribute{
				MarkdownDescription: "The number of processes to run.",
				Required:            true,
			},
			"startsecs": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait before starting the process. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"stopwaitsecs": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait before stopping the process. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"stopsignal": schema.StringAttribute{
				MarkdownDescription: "The signal to send to stop the process (e.g. `SIGTERM`). Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "The supervisor configuration of the background process. Update-only (write-only; not returned by the API).",
				Optional:            true,
			},
			"status":     schema.StringAttribute{MarkdownDescription: "The status of the background process.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *serverBackgroundProcessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverBackgroundProcessResource) basePath(m *serverBackgroundProcessResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/background-processes", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *serverBackgroundProcessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverBackgroundProcessResourceModel
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
	if !plan.StartSecs.IsNull() && !plan.StartSecs.IsUnknown() {
		body["startsecs"] = plan.StartSecs.ValueInt64()
	}
	if !plan.StopWaitSecs.IsNull() && !plan.StopWaitSecs.IsUnknown() {
		body["stopwaitsecs"] = plan.StopWaitSecs.ValueInt64()
	}
	if !plan.StopSignal.IsNull() && !plan.StopSignal.IsUnknown() {
		body["stopsignal"] = plan.StopSignal.ValueString()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge background process", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected background process ID", fmt.Sprintf("Forge returned a non-numeric background process ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read background process after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverBackgroundProcessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverBackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge background process", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverBackgroundProcessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverBackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"name": plan.Name.ValueString()}
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		body["config"] = plan.Config.ValueString()
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge background process", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read background process after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverBackgroundProcessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverBackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge background process", err.Error())
	}
}

// ImportState accepts "organization/server_id/background_process_id".
func (r *serverBackgroundProcessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/background_process_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	processID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and background_process_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), processID)...)
}

// readInto GETs the background process identified by m and fills the
// computed/optional fields the read endpoint returns. The create-only inputs
// (name, site_id, startsecs, stopwaitsecs, stopsignal) and update-only config
// are not part of the read shape, so they are left untouched.
func (r *serverBackgroundProcessResource) readInto(ctx context.Context, m *serverBackgroundProcessResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a serverBackgroundProcessAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Type = types.StringValue("backgroundProcesses")
	m.Command = types.StringValue(a.Command)
	m.User = types.StringValue(a.User)
	m.Directory = types.StringPointerValue(a.Directory)
	m.Processes = types.Int64Value(a.Processes)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	return nil
}
