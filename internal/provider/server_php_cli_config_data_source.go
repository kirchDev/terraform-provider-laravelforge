package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Data source: php.ini CLI config for a given PHP version on a server. ---

var (
	_ datasource.DataSource              = (*serverPHPCLIConfigDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPHPCLIConfigDataSource)(nil)
)

// NewServerPhpCliConfigDataSource returns a new laravelforge_server_php_cli_config data source.
func NewServerPhpCliConfigDataSource() datasource.DataSource {
	return &serverPHPCLIConfigDataSource{}
}

type serverPHPCLIConfigDataSource struct {
	client *client.Client
}

type serverPHPCLIConfigDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	PHPVersion    types.Int64  `tfsdk:"php_version"`
	Configuration types.String `tfsdk:"configuration"`
}

func (d *serverPHPCLIConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_cli_config"
}

func (d *serverPHPCLIConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the php.ini CLI configuration for a given PHP version on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization":  schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":     schema.Int64Attribute{MarkdownDescription: "ID of the server.", Required: true},
			"php_version":   schema.Int64Attribute{MarkdownDescription: "ID of the PHP version whose CLI config is fetched.", Required: true},
			"configuration": schema.StringAttribute{MarkdownDescription: "php.ini CLI configuration as reported by Forge.", Computed: true},
		},
	}
}

func (d *serverPHPCLIConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPHPCLIConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPHPCLIConfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions/%d/configs/cli",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.PHPVersion.ValueInt64())
	var a serverPHPCLIConfigAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge PHP CLI config", err.Error())
		return
	}

	data.Configuration = types.StringValue(a.Configuration)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
