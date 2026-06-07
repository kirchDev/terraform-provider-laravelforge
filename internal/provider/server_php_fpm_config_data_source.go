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
	_ datasource.DataSource              = (*serverPHPFpmConfigDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPHPFpmConfigDataSource)(nil)
)

// NewServerPHPFpmConfigDataSource returns a new laravelforge_server_php_fpm_config data source.
func NewServerPHPFpmConfigDataSource() datasource.DataSource {
	return &serverPHPFpmConfigDataSource{}
}

type serverPHPFpmConfigDataSource struct {
	client *client.Client
}

type serverPHPFpmConfigDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	PHPVersion    types.String `tfsdk:"php_version"`
	Configuration types.String `tfsdk:"configuration"`
}

func (d *serverPHPFpmConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_fpm_config"
}

func (d *serverPHPFpmConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the php-fpm configuration for a given PHP version on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization":  schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":     schema.Int64Attribute{MarkdownDescription: "ID of the server the PHP version belongs to.", Required: true},
			"php_version":   schema.StringAttribute{MarkdownDescription: "PHP version key whose php-fpm configuration is fetched (e.g. `php82`).", Required: true},
			"configuration": schema.StringAttribute{MarkdownDescription: "The php-fpm configuration content.", Computed: true},
		},
	}
}

func (d *serverPHPFpmConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPHPFpmConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPHPFpmConfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions/%s/configs/fpm",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.PHPVersion.ValueString())
	var a serverPHPFpmConfigAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge php-fpm configuration", err.Error())
		return
	}

	data.Configuration = types.StringValue(a.Configuration)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
