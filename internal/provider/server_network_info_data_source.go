package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ datasource.DataSource              = (*serverNetworkInfoDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverNetworkInfoDataSource)(nil)
)

// NewServerNetworkInfoDataSource returns a new laravelforge_server_network_info data source.
func NewServerNetworkInfoDataSource() datasource.DataSource {
	return &serverNetworkInfoDataSource{}
}

type serverNetworkInfoDataSource struct {
	client *client.Client
}

// serverNetworkInfoMember mirrors the subset of a member ServerResource's
// JSON:API "attributes" we care about: its numeric id.
type serverNetworkInfoMember struct {
	ID int64 `json:"id"`
}

type serverNetworkInfoDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	Servers      types.Set    `tfsdk:"servers"`
	ServerCount  types.Int64  `tfsdk:"server_count"`
}

func (d *serverNetworkInfoDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_network_info"
}

func (d *serverNetworkInfoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the network membership of a Laravel Forge server (singleton per server): the set of servers that share this server's private network. Read-only companion to the `laravelforge_server_network` resource.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server whose network membership is read.", Required: true},
			"servers":      schema.SetAttribute{MarkdownDescription: "Numeric IDs of the servers in this server's network.", ElementType: types.Int64Type, Computed: true},
			"server_count": schema.Int64Attribute{MarkdownDescription: "Number of servers in this server's network.", Computed: true},
		},
	}
}

func (d *serverNetworkInfoDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverNetworkInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverNetworkInfoDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/network", data.Organization.ValueString(), data.ServerID.ValueInt64())
	members, err := d.client.List(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server network", err.Error())
		return
	}

	ids := make([]int64, 0, len(members))
	for _, raw := range members {
		var m serverNetworkInfoMember
		if len(raw.Attributes) > 0 {
			if err := json.Unmarshal(raw.Attributes, &m); err != nil {
				resp.Diagnostics.AddError("Unable to decode Forge server network member", err.Error())
				return
			}
		}
		if m.ID == 0 {
			// Fall back to the resource-level string id when attributes are absent.
			if parsed, perr := strconv.ParseInt(raw.ID, 10, 64); perr == nil {
				m.ID = parsed
			}
		}
		if m.ID != 0 {
			ids = append(ids, m.ID)
		}
	}

	set, diags := types.SetValueFrom(ctx, types.Int64Type, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Servers = set
	data.ServerCount = types.Int64Value(int64(len(ids)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
