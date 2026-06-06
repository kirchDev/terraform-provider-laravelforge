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

// laravelforge_server_network data source: reads a server's private-network
// membership (the set of other server IDs it can reach). Reuses the resource's
// serverNetworkMemberAttributes for the member id shape.

var (
	_ datasource.DataSource              = (*serverNetworkDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverNetworkDataSource)(nil)
)

// NewServerNetworkDataSource returns a new laravelforge_server_network data source.
func NewServerNetworkDataSource() datasource.DataSource {
	return &serverNetworkDataSource{}
}

type serverNetworkDataSource struct {
	client *client.Client
}

type serverNetworkDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	Servers      types.Set    `tfsdk:"servers"`
}

func (d *serverNetworkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_network"
}

func (d *serverNetworkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the private-network membership of a Laravel Forge server: the set of other server IDs it can reach.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server whose network membership is read.", Required: true},
			"servers": schema.SetAttribute{
				MarkdownDescription: "Set of server IDs currently in this server's network.",
				ElementType:         types.Int64Type,
				Computed:            true,
			},
		},
	}
}

func (d *serverNetworkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverNetworkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverNetworkDataSourceModel
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
		var a serverNetworkMemberAttributes
		if len(raw.Attributes) > 0 {
			if err := json.Unmarshal(raw.Attributes, &a); err != nil {
				resp.Diagnostics.AddError("Unable to decode server network member", err.Error())
				return
			}
		}
		if a.ID == 0 {
			if parsed, perr := strconv.ParseInt(raw.ID, 10, 64); perr == nil {
				a.ID = parsed
			}
		}
		if a.ID != 0 {
			ids = append(ids, a.ID)
		}
	}

	set, diags := serverNetworkIDsToSet(ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Servers = set

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
