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

// Singleton resource: the load-balancing node set of a balancer site. There is
// no own id — identity is the parent ids (organization/server_id/site_id). The
// GET index confirms existence; Create and Update both PUT the singleton path
// (create == PUT). Delete is a no-op (the node set lives with the balancer site,
// which is owned by the site resource). Per the modeling rule we map only the
// scalar request fields here (balancer_method, balancer_keepalive_max_connections);
// the per-node "balancing" array is skipped this pass.

var (
	_ resource.Resource                = (*siteLoadBalancingResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteLoadBalancingResource)(nil)
	_ resource.ResourceWithImportState = (*siteLoadBalancingResource)(nil)
)

// NewSiteLoadBalancingResource returns a new laravelforge_site_load_balancing resource.
func NewSiteLoadBalancingResource() resource.Resource {
	return &siteLoadBalancingResource{}
}

type siteLoadBalancingResource struct {
	client *client.Client
}

type siteLoadBalancingResourceModel struct {
	Organization                    types.String `tfsdk:"organization"`
	ServerID                        types.Int64  `tfsdk:"server_id"`
	SiteID                          types.Int64  `tfsdk:"site_id"`
	BalancerMethod                  types.String `tfsdk:"balancer_method"`
	BalancerKeepaliveMaxConnections types.Int64  `tfsdk:"balancer_keepalive_max_connections"`
}

func (r *siteLoadBalancingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_load_balancing"
}

func (r *siteLoadBalancingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the load-balancing node set of a Laravel Forge balancer site (singleton per site).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server that owns the balancer site.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the balancer site.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"balancer_method": schema.StringAttribute{
				MarkdownDescription: "Load-balancing method. One of `round_robin`, `least_conn`, `ip_hash`.",
				Required:            true,
			},
			"balancer_keepalive_max_connections": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of keepalive connections (0-256).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteLoadBalancingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteLoadBalancingResource) singletonPath(m *siteLoadBalancingResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/load-balancing-nodes", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

// put sends the load-balancer configuration. create == update for a singleton.
func (r *siteLoadBalancingResource) put(ctx context.Context, m *siteLoadBalancingResourceModel) error {
	body := map[string]any{"balancer_method": m.BalancerMethod.ValueString()}
	if !m.BalancerKeepaliveMaxConnections.IsNull() && !m.BalancerKeepaliveMaxConnections.IsUnknown() {
		body["balancer_keepalive_max_connections"] = m.BalancerKeepaliveMaxConnections.ValueInt64()
	}
	_, err := r.client.Write(ctx, "PUT", r.singletonPath(m), body, nil)
	return err
}

func (r *siteLoadBalancingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteLoadBalancingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge load-balancing nodes", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteLoadBalancingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteLoadBalancingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The GET returns the node collection; it doesn't echo the balancer_method /
	// keepalive config, so existence is all we can confirm here. A 404 (site or
	// balancer gone) drops the resource from state.
	if _, err := r.client.List(ctx, r.singletonPath(&state)); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge load-balancing nodes", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteLoadBalancingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteLoadBalancingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge load-balancing nodes", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the node set has no own destroy endpoint; it lives with the
// balancer site, which is removed by deleting the site resource itself.
func (r *siteLoadBalancingResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteLoadBalancingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
