package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Data source: default site/FPM PHP version for a Laravel Forge server. ---

var (
	_ datasource.DataSource              = (*serverPHPSiteVersionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverPHPSiteVersionDataSource)(nil)
)

// NewServerPhpSiteVersionDataSource returns a new laravelforge_server_php_site_version data source.
func NewServerPhpSiteVersionDataSource() datasource.DataSource {
	return &serverPHPSiteVersionDataSource{}
}

type serverPHPSiteVersionDataSource struct {
	client *client.Client
}

type serverPHPSiteVersionDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	Version      types.String `tfsdk:"version"`
	BinaryName   types.String `tfsdk:"binary_name"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *serverPHPSiteVersionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_site_version"
}

func (d *serverPHPSiteVersionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the default site/FPM PHP version for a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server whose default site PHP version is fetched.", Required: true},
			"version":      schema.StringAttribute{MarkdownDescription: "PHP version as reported by Forge.", Computed: true},
			"binary_name":  schema.StringAttribute{MarkdownDescription: "PHP binary name (e.g. `php82`).", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "PHP version status.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *serverPHPSiteVersionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverPHPSiteVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverPHPSiteVersionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/php/site-version",
		data.Organization.ValueString(), data.ServerID.ValueInt64())
	var a serverPHPSiteVersionAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge default site PHP version", err.Error())
		return
	}

	data.Version = types.StringValue(a.Version)
	data.BinaryName = types.StringValue(a.BinaryName)
	data.Status = types.StringValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
