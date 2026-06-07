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
	_ datasource.DataSource              = (*serverPHPMaxExecutionTimeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPHPMaxExecutionTimeDataSource)(nil)
)

// NewServerPHPMaxExecutionTimeDataSource returns a new
// laravelforge_server_php_max_execution_time data source.
func NewServerPHPMaxExecutionTimeDataSource() datasource.DataSource {
	return &serverPHPMaxExecutionTimeDataSource{}
}

type serverPHPMaxExecutionTimeDataSource struct {
	client *client.Client
}

func (d *serverPHPMaxExecutionTimeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_max_execution_time"
}

func (d *serverPHPMaxExecutionTimeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the PHP `max_execution_time` setting of a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization":       schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":          schema.Int64Attribute{MarkdownDescription: "ID of the server.", Required: true},
			"max_execution_time": schema.Int64Attribute{MarkdownDescription: "PHP `max_execution_time` in seconds.", Computed: true},
		},
	}
}

func (d *serverPHPMaxExecutionTimeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPHPMaxExecutionTimeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPHPMaxExecutionTimeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/max-execution-time", data.Organization.ValueString(), data.ServerID.ValueInt64())
	var a serverPHPMaxExecutionTimeAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server PHP max execution time", err.Error())
		return
	}

	data.MaxExecutionTime = types.Int64PointerValue(a.MaxExecutionTime)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
