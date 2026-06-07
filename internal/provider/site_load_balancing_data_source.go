package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ datasource.DataSource              = (*siteLoadBalancingDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteLoadBalancingDataSource)(nil)
)

// NewSiteLoadBalancingDataSource returns a new laravelforge_site_load_balancing data source.
func NewSiteLoadBalancingDataSource() datasource.DataSource {
	return &siteLoadBalancingDataSource{}
}

type siteLoadBalancingDataSource struct {
	client *client.Client
}

type siteLoadBalancingDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	NodeCount    types.Int64  `tfsdk:"node_count"`
}

func (d *siteLoadBalancingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_load_balancing"
}

func (d *siteLoadBalancingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the load-balancing node set of a Laravel Forge balancer site (singleton per site). The per-node list is not surfaced this pass; `node_count` reports how many balancing nodes are configured.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server that owns the balancer site.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "ID of the balancer site.", Required: true},
			"node_count":   schema.Int64Attribute{MarkdownDescription: "Number of load-balancing nodes configured for the site.", Computed: true},
		},
	}
}

func (d *siteLoadBalancingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteLoadBalancingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteLoadBalancingDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/load-balancing-nodes", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	nodes, err := d.client.List(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge load-balancing nodes", err.Error())
		return
	}

	data.NodeCount = types.Int64Value(int64(len(nodes)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
