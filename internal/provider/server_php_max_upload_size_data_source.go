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
	_ datasource.DataSource              = (*serverPHPMaxUploadSizeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPHPMaxUploadSizeDataSource)(nil)
)

// NewServerPhpMaxUploadSizeDataSource returns a new laravelforge_server_php_max_upload_size data source.
func NewServerPhpMaxUploadSizeDataSource() datasource.DataSource {
	return &serverPHPMaxUploadSizeDataSource{}
}

type serverPHPMaxUploadSizeDataSource struct {
	client *client.Client
}

type serverPHPMaxUploadSizeDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	MaxUploadSize types.Int64  `tfsdk:"max_upload_size"`
}

func (d *serverPHPMaxUploadSizeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_max_upload_size"
}

func (d *serverPHPMaxUploadSizeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the PHP max upload size (`upload_max_filesize` / `post_max_size`) for a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization":    schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":       schema.Int64Attribute{MarkdownDescription: "ID of the server whose PHP max upload size is fetched.", Required: true},
			"max_upload_size": schema.Int64Attribute{MarkdownDescription: "Maximum upload size in megabytes.", Computed: true},
		},
	}
}

func (d *serverPHPMaxUploadSizeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPHPMaxUploadSizeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPHPMaxUploadSizeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/max-upload-size",
		data.Organization.ValueString(), data.ServerID.ValueInt64())
	var a serverPHPMaxUploadSizeAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge PHP max upload size", err.Error())
		return
	}

	data.MaxUploadSize = types.Int64PointerValue(a.MaxUploadSize)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
