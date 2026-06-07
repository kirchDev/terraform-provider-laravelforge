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
	_ datasource.DataSource              = (*serverPhpPoolConfigDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPhpPoolConfigDataSource)(nil)
)

// NewServerPhpPoolConfigDataSource returns a new laravelforge_server_php_pool_config data source.
func NewServerPhpPoolConfigDataSource() datasource.DataSource {
	return &serverPhpPoolConfigDataSource{}
}

type serverPhpPoolConfigDataSource struct {
	client *client.Client
}

type serverPhpPoolConfigDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	PHPVersionID  types.Int64  `tfsdk:"php_version_id"`
	Configuration types.String `tfsdk:"configuration"`
}

func (d *serverPhpPoolConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_pool_config"
}

func (d *serverPhpPoolConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the php-fpm pool configuration of a PHP version on a Laravel Forge server (singleton per PHP version).",
		Attributes: map[string]schema.Attribute{
			"organization":   schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":      schema.Int64Attribute{MarkdownDescription: "ID of the server the PHP version belongs to.", Required: true},
			"php_version_id": schema.Int64Attribute{MarkdownDescription: "ID of the PHP version whose pool config is fetched.", Required: true},
			"configuration":  schema.StringAttribute{MarkdownDescription: "The current php-fpm pool configuration as returned by Forge.", Computed: true},
		},
	}
}

func (d *serverPhpPoolConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPhpPoolConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPhpPoolConfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions/%d/configs/pool", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.PHPVersionID.ValueInt64())
	var a serverPhpPoolConfigAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge PHP pool config", err.Error())
		return
	}

	data.Configuration = types.StringValue(a.Configuration)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
