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
	_ resource.Resource                = (*daemonResource)(nil)
	_ resource.ResourceWithConfigure   = (*daemonResource)(nil)
	_ resource.ResourceWithImportState = (*daemonResource)(nil)
)

// NewDaemonResource returns a new laravelforge_daemon resource.
func NewDaemonResource() resource.Resource {
	return &daemonResource{}
}

type daemonResource struct {
	client *client.Client
}

type daemonResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Command      types.String `tfsdk:"command"`
	User         types.String `tfsdk:"user"`
	Directory    types.String `tfsdk:"directory"`
	Processes    types.Int64  `tfsdk:"processes"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *daemonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_daemon"
}

func (r *daemonResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a daemon (background process) on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to create the daemon on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the daemon.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"command": schema.StringAttribute{
				MarkdownDescription: "Command the daemon runs. Changing it recreates the daemon.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "User account the command runs under. Changing it recreates the daemon.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"directory": schema.StringAttribute{
				MarkdownDescription: "Working directory the command runs in. Changing it recreates the daemon.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"processes": schema.Int64Attribute{
				MarkdownDescription: "Number of processes to run.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"status":     schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *daemonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *daemonResource) basePath(m *daemonResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/background-processes", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *daemonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan daemonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"command":   plan.Command.ValueString(),
		"user":      plan.User.ValueString(),
		"directory": plan.Directory.ValueString(),
	}
	if !plan.Processes.IsNull() && !plan.Processes.IsUnknown() {
		body["processes"] = plan.Processes.ValueInt64()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge daemon", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected daemon ID", fmt.Sprintf("Forge returned a non-numeric daemon ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read daemon after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *daemonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state daemonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge daemon", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *daemonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan daemonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{}
	if !plan.Processes.IsNull() && !plan.Processes.IsUnknown() {
		body["processes"] = plan.Processes.ValueInt64()
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge daemon", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read daemon after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *daemonResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state daemonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge daemon", err.Error())
	}
}

// ImportState accepts "organization/server_id/daemon_id".
func (r *daemonResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/daemon_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	daemonID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and daemon_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), daemonID)...)
}

// readInto GETs the daemon identified by m and fills the computed fields.
// Single-daemon reads are server-scoped (the org-scoped path 404s).
func (r *daemonResource) readInto(ctx context.Context, m *daemonResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a daemonAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Command = types.StringValue(a.Command)
	m.User = types.StringValue(a.User)
	m.Directory = types.StringValue(a.Directory)
	m.Processes = types.Int64Value(a.Processes)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	return nil
}
