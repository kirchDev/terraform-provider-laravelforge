package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// laravelforge_server_network manages a server's network/firewall membership:
// the set of other server IDs that this server can reach over the private
// network. It is a singleton per server — identity is the parent ids
// (organization + server_id), there is no own id.
//
//   - Read  : GET  .../servers/{server}/network -> JSON:API collection of the
//     ServerResource members; we extract each member's numeric attributes.id.
//   - Create/Update: PUT .../servers/{server}/network with a FLAT body
//     {"servers": [..ids..]} (202 Accepted, empty body — re-read for state).
//   - Delete: no destroy endpoint exists; we PUT an empty membership set so the
//     network is emptied when the resource is removed.

var (
	_ resource.Resource                = (*serverNetworkResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverNetworkResource)(nil)
	_ resource.ResourceWithImportState = (*serverNetworkResource)(nil)
)

// NewServerNetworkResource returns a new laravelforge_server_network resource.
func NewServerNetworkResource() resource.Resource {
	return &serverNetworkResource{}
}

type serverNetworkResource struct {
	client *client.Client
}

// serverNetworkMemberAttributes mirrors the subset of a member ServerResource's
// JSON:API "attributes" we care about: its numeric id.
type serverNetworkMemberAttributes struct {
	ID int64 `json:"id"`
}

type serverNetworkResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	Servers      types.Set    `tfsdk:"servers"`
}

func (r *serverNetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_network"
}

func (r *serverNetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the private-network membership of a Laravel Forge server: the set of other server IDs it can reach. Singleton per server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server whose network membership is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"servers": schema.SetAttribute{
				MarkdownDescription: "Set of server IDs that should be in this server's network.",
				ElementType:         types.Int64Type,
				Required:            true,
			},
		},
	}
}

func (r *serverNetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverNetworkResource) networkPath(m *serverNetworkResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/network", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

// writeMembership PUTs the desired set of member server IDs (flat body).
func (r *serverNetworkResource) writeMembership(ctx context.Context, m *serverNetworkResourceModel, ids []int64) error {
	body := map[string]any{"servers": ids}
	_, err := r.client.Write(ctx, "PUT", r.networkPath(m), body, nil)
	return err
}

func (r *serverNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ids, diags := serverNetworkSetToIDs(ctx, plan.Servers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.writeMembership(ctx, &plan, ids); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge server network", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read server network after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge server network", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ids, diags := serverNetworkSetToIDs(ctx, plan.Servers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.writeMembership(ctx, &plan, ids); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge server network", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read server network after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete empties the network. There is no destroy endpoint for this singleton;
// PUT-ing an empty membership set is the closest equivalent to removal.
func (r *serverNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.writeMembership(ctx, &state, []int64{}); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to clear Forge server network", err.Error())
	}
}

// ImportState accepts "organization/server_id".
func (r *serverNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the network membership collection for m and fills m.Servers
// with the member server IDs.
func (r *serverNetworkResource) readInto(ctx context.Context, m *serverNetworkResourceModel) error {
	members, err := r.client.List(ctx, r.networkPath(m))
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(members))
	for _, raw := range members {
		var a serverNetworkMemberAttributes
		if len(raw.Attributes) > 0 {
			if err := json.Unmarshal(raw.Attributes, &a); err != nil {
				return fmt.Errorf("decoding server network member attributes: %w", err)
			}
		}
		if a.ID == 0 {
			// Fall back to the resource-level string id when attributes are absent.
			if parsed, perr := strconv.ParseInt(raw.ID, 10, 64); perr == nil {
				a.ID = parsed
			}
		}
		if a.ID != 0 {
			ids = append(ids, a.ID)
		}
	}

	set, diags := serverNetworkIDsToSet(ids)
	if diags.HasError() {
		return fmt.Errorf("building server network set: %s", diags.Errors())
	}
	m.Servers = set
	return nil
}

// serverNetworkSetToIDs extracts the int64 server IDs from a types.Set.
func serverNetworkSetToIDs(ctx context.Context, s types.Set) ([]int64, diag.Diagnostics) {
	var ids []int64
	if s.IsNull() || s.IsUnknown() {
		return []int64{}, nil
	}
	diags := s.ElementsAs(ctx, &ids, false)
	if ids == nil {
		ids = []int64{}
	}
	return ids, diags
}

// serverNetworkIDsToSet builds a types.Set of int64 from a slice of server IDs.
func serverNetworkIDsToSet(ids []int64) (types.Set, diag.Diagnostics) {
	return types.SetValueFrom(context.Background(), types.Int64Type, ids)
}
