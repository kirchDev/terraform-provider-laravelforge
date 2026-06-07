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
	_ resource.Resource                = (*serverPHPMaxExecutionTimeResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPMaxExecutionTimeResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPMaxExecutionTimeResource)(nil)
)

// NewServerPHPMaxExecutionTimeResource returns a new
// laravelforge_server_php_max_execution_time resource.
func NewServerPHPMaxExecutionTimeResource() resource.Resource {
	return &serverPHPMaxExecutionTimeResource{}
}

type serverPHPMaxExecutionTimeResource struct {
	client *client.Client
}

// serverPHPMaxExecutionTimeAttributes mirrors the JSON:API "attributes" of the
// PhpMaxExecutionTimeResource.
type serverPHPMaxExecutionTimeAttributes struct {
	MaxExecutionTime *int64 `json:"max_execution_time"`
}

type serverPHPMaxExecutionTimeModel struct {
	Organization     types.String `tfsdk:"organization"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	MaxExecutionTime types.Int64  `tfsdk:"max_execution_time"`
}

func (r *serverPHPMaxExecutionTimeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_max_execution_time"
}

func (r *serverPHPMaxExecutionTimeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the PHP `max_execution_time` setting on a Laravel Forge server. This is a singleton per server: it has no own ID, cannot be destroyed (delete is a no-op), and apply just PUTs the desired value.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server whose PHP max execution time is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"max_execution_time": schema.Int64Attribute{
				MarkdownDescription: "PHP `max_execution_time` in seconds.",
				Required:            true,
			},
		},
	}
}

func (r *serverPHPMaxExecutionTimeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPHPMaxExecutionTimeResource) path(m *serverPHPMaxExecutionTimeModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/max-execution-time", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

// write PUTs the desired max_execution_time and reads it back into m.
func (r *serverPHPMaxExecutionTimeResource) write(ctx context.Context, m *serverPHPMaxExecutionTimeModel) error {
	body := map[string]any{"max_execution_time": m.MaxExecutionTime.ValueInt64()}
	if _, err := r.client.Write(ctx, "PUT", r.path(m), body, nil); err != nil {
		return err
	}
	return r.readInto(ctx, m)
}

// readInto GETs the singleton and fills max_execution_time.
func (r *serverPHPMaxExecutionTimeResource) readInto(ctx context.Context, m *serverPHPMaxExecutionTimeModel) error {
	var a serverPHPMaxExecutionTimeAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.MaxExecutionTime = types.Int64PointerValue(a.MaxExecutionTime)
	return nil
}

func (r *serverPHPMaxExecutionTimeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPMaxExecutionTimeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge server PHP max execution time", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPMaxExecutionTimeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPMaxExecutionTimeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge server PHP max execution time", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPHPMaxExecutionTimeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPMaxExecutionTimeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge server PHP max execution time", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the PHP max execution time is a server-level singleton
// setting with no destroy semantics.
func (r *serverPHPMaxExecutionTimeResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id".
func (r *serverPHPMaxExecutionTimeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id\".")
		return
	}
	serverID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
}
