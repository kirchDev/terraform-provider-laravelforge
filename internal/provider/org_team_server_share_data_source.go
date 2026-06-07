package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Data source: looks up a single server shared with a team. ---
//
// Reads the team's servers index and returns the entry whose id matches
// server_id. Reuses orgTeamServerShareAttributes from the resource file.

var (
	_ datasource.DataSource              = (*orgTeamServerShareDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgTeamServerShareDataSource)(nil)
)

// NewOrgTeamServerShareDataSource returns a new laravelforge_org_team_server_share data source.
func NewOrgTeamServerShareDataSource() datasource.DataSource {
	return &orgTeamServerShareDataSource{}
}

type orgTeamServerShareDataSource struct {
	client *client.Client
}

func (d *orgTeamServerShareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_server_share"
}

func (d *orgTeamServerShareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge server shared with a team by server ID.",
		Attributes: map[string]schema.Attribute{
			"organization":       schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"team_id":            schema.Int64Attribute{MarkdownDescription: "ID of the team the server is shared with.", Required: true},
			"server_id":          schema.Int64Attribute{MarkdownDescription: "ID of the shared server.", Required: true},
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

func (d *orgTeamServerShareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = c
}

func (d *orgTeamServerShareDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgTeamServerShareModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	indexPath := fmt.Sprintf("/api/orgs/%s/teams/%d/servers", data.Organization.ValueString(), data.TeamID.ValueInt64())
	resources, err := d.client.List(ctx, indexPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge team server share", err.Error())
		return
	}

	want := data.ServerID.ValueInt64()
	var found *orgTeamServerShareAttributes
	for _, res := range resources {
		if len(res.Attributes) == 0 {
			continue
		}
		var a orgTeamServerShareAttributes
		if err := json.Unmarshal(res.Attributes, &a); err != nil {
			resp.Diagnostics.AddError("Unable to decode Forge team server share", err.Error())
			return
		}
		if a.ID == want {
			found = &a
			break
		}
	}
	if found == nil {
		resp.Diagnostics.AddError("Forge team server share not found", fmt.Sprintf("Server %d is not shared with team %d.", want, data.TeamID.ValueInt64()))
		return
	}

	applyOrgTeamServerShareAttrs(&data, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
