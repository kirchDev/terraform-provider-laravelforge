package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Data source: server PHP OPcache status (singleton). ---

var (
	_ datasource.DataSource              = (*serverPHPOpcacheDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPHPOpcacheDataSource)(nil)
)

// NewServerPhpOpcacheDataSource returns a new laravelforge_server_php_opcache data source.
func NewServerPhpOpcacheDataSource() datasource.DataSource {
	return &serverPHPOpcacheDataSource{}
}

type serverPHPOpcacheDataSource struct {
	client *client.Client
}

func (d *serverPHPOpcacheDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_opcache"
}

func (d *serverPHPOpcacheDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the PHP OPcache status of a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization":    schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":       schema.Int64Attribute{MarkdownDescription: "ID of the server.", Required: true},
			"opcache_enabled": schema.BoolAttribute{MarkdownDescription: "Whether PHP OPcache is enabled on the server.", Computed: true},
		},
	}
}

func (d *serverPHPOpcacheDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPHPOpcacheDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPHPOpcacheModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/opcache", data.Organization.ValueString(), data.ServerID.ValueInt64())
	var a serverPHPOpcacheAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge PHP OPcache", err.Error())
		return
	}

	data.OpcacheEnabled = types.BoolValue(a.OpcacheEnabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
