package provider

import (
	"context"
	"encoding/json"
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

// --- Link resource: shares an existing server with a team. ---
//
// There is no PUT/PATCH for a share, so every writable input
// (organization, team_id, server_id) is RequiresReplace: a change tears the
// share down and re-creates it. Create POSTs the flat ShareServerRequest
// ({server_id}) to the team's servers index; Read GETs that same index and
// looks for the shared server by id; Delete unshares via
// .../servers/{server_id}.

var (
	_ resource.Resource                = (*orgTeamServerShareResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgTeamServerShareResource)(nil)
	_ resource.ResourceWithImportState = (*orgTeamServerShareResource)(nil)
)

// NewOrgTeamServerShareResource returns a new laravelforge_org_team_server_share resource.
func NewOrgTeamServerShareResource() resource.Resource {
	return &orgTeamServerShareResource{}
}

type orgTeamServerShareResource struct {
	client *client.Client
}

// orgTeamServerShareAttributes mirrors the scalar JSON:API "attributes" of the
// ServerResource returned by the share/index endpoints.
type orgTeamServerShareAttributes struct {
	ID               int64   `json:"id"`
	CredentialID     *int64  `json:"credential_id"`
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Type             string  `json:"type"`
	UbuntuVersion    *string `json:"ubuntu_version"`
	SSHPort          int64   `json:"ssh_port"`
	Provider         string  `json:"provider"`
	Identifier       *string `json:"identifier"`
	Size             string  `json:"size"`
	Region           string  `json:"region"`
	PHPVersion       *string `json:"php_version"`
	PHPCLIVersion    *string `json:"php_cli_version"`
	OpcacheStatus    *string `json:"opcache_status"`
	DatabaseType     *string `json:"database_type"`
	DBStatus         *string `json:"db_status"`
	RedisStatus      *string `json:"redis_status"`
	IPAddress        *string `json:"ip_address"`
	PrivateIPAddress *string `json:"private_ip_address"`
	Revoked          bool    `json:"revoked"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	ConnectionStatus string  `json:"connection_status"`
	Timezone         string  `json:"timezone"`
	LocalPublicKey   *string `json:"local_public_key"`
	IsReady          bool    `json:"is_ready"`
}

type orgTeamServerShareModel struct {
	Organization     types.String `tfsdk:"organization"`
	TeamID           types.Int64  `tfsdk:"team_id"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	Type             types.String `tfsdk:"type"`
	CloudProvider    types.String `tfsdk:"cloud_provider"`
	Region           types.String `tfsdk:"region"`
	Size             types.String `tfsdk:"size"`
	IPAddress        types.String `tfsdk:"ip_address"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	PHPVersion       types.String `tfsdk:"php_version"`
	IsReady          types.Bool   `tfsdk:"is_ready"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *orgTeamServerShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_server_share"
}

func (r *orgTeamServerShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shares an existing Laravel Forge server with a team. This is a link between a team and a server; there is no update path, so any change re-creates the share.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the team to share the server with.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to share with the team.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name":               schema.StringAttribute{MarkdownDescription: "Server name.", Computed: true},
			"slug":               schema.StringAttribute{MarkdownDescription: "Server slug.", Computed: true},
			"type":               schema.StringAttribute{MarkdownDescription: "Server type (e.g. `app`, `web`, `database`).", Computed: true},
			"cloud_provider":     schema.StringAttribute{MarkdownDescription: "Underlying server provider (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
			"region":             schema.StringAttribute{MarkdownDescription: "Region the server runs in.", Computed: true},
			"size":               schema.StringAttribute{MarkdownDescription: "Server size / plan.", Computed: true},
			"ip_address":         schema.StringAttribute{MarkdownDescription: "Public IP address.", Computed: true},
			"private_ip_address": schema.StringAttribute{MarkdownDescription: "Private IP address.", Computed: true},
			"php_version":        schema.StringAttribute{MarkdownDescription: "Installed PHP version.", Computed: true},
			"is_ready":           schema.BoolAttribute{MarkdownDescription: "Whether the server has finished provisioning.", Computed: true},
			"created_at":         schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *orgTeamServerShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *orgTeamServerShareResource) indexPath(m *orgTeamServerShareModel) string {
	return fmt.Sprintf("/api/orgs/%s/teams/%d/servers", m.Organization.ValueString(), m.TeamID.ValueInt64())
}

func (r *orgTeamServerShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgTeamServerShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"server_id": plan.ServerID.ValueInt64()}
	var a orgTeamServerShareAttributes
	if _, err := r.client.Write(ctx, "POST", r.indexPath(&plan), body, &a); err != nil {
		resp.Diagnostics.AddError("Unable to share Forge server with team", err.Error())
		return
	}
	applyOrgTeamServerShareAttrs(&plan, &a)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamServerShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgTeamServerShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	a, err := r.findShared(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge team server share", err.Error())
		return
	}
	if a == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	applyOrgTeamServerShareAttrs(&state, a)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable: every input is RequiresReplace and there is no update
// endpoint. It exists only to satisfy the resource.Resource interface.
func (r *orgTeamServerShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgTeamServerShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamServerShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgTeamServerShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.indexPath(&state), state.ServerID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to unshare Forge server from team", err.Error())
	}
}

// ImportState accepts "organization/team_id/server_id".
func (r *orgTeamServerShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/team_id/server_id\".")
		return
	}
	teamID, err1 := strconv.ParseInt(parts[1], 10, 64)
	serverID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "team_id and server_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), teamID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
}

// findShared lists the team's servers and returns the shared server matching
// m.ServerID, or nil if it is no longer shared with the team.
func (r *orgTeamServerShareResource) findShared(ctx context.Context, m *orgTeamServerShareModel) (*orgTeamServerShareAttributes, error) {
	resources, err := r.client.List(ctx, r.indexPath(m))
	if err != nil {
		if client.NotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	want := m.ServerID.ValueInt64()
	for _, res := range resources {
		var a orgTeamServerShareAttributes
		if len(res.Attributes) == 0 {
			continue
		}
		if err := json.Unmarshal(res.Attributes, &a); err != nil {
			return nil, fmt.Errorf("decoding shared server attributes: %w", err)
		}
		if a.ID == want {
			return &a, nil
		}
	}
	return nil, nil
}

func applyOrgTeamServerShareAttrs(m *orgTeamServerShareModel, a *orgTeamServerShareAttributes) {
	m.ServerID = types.Int64Value(a.ID)
	m.Name = types.StringValue(a.Name)
	m.Slug = types.StringValue(a.Slug)
	m.Type = types.StringValue(a.Type)
	m.CloudProvider = types.StringValue(a.Provider)
	m.Region = types.StringValue(a.Region)
	m.Size = types.StringValue(a.Size)
	m.IPAddress = types.StringPointerValue(a.IPAddress)
	m.PrivateIPAddress = types.StringPointerValue(a.PrivateIPAddress)
	m.PHPVersion = types.StringPointerValue(a.PHPVersion)
	m.IsReady = types.BoolValue(a.IsReady)
	m.CreatedAt = types.StringValue(a.CreatedAt)
}
