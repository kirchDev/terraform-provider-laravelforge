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
	_ datasource.DataSource              = (*siteEnvironmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteEnvironmentDataSource)(nil)
)

// NewSiteEnvironmentDataSource returns a new laravelforge_site_environment data source.
func NewSiteEnvironmentDataSource() datasource.DataSource {
	return &siteEnvironmentDataSource{}
}

type siteEnvironmentDataSource struct {
	client *client.Client
}

type siteEnvironmentDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Content      types.String `tfsdk:"content"`
}

func (d *siteEnvironmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_environment"
}

func (d *siteEnvironmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the `.env` file contents of a Laravel Forge site (singleton per site).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "ID of the site whose environment is fetched.", Required: true},
			"content":      schema.StringAttribute{MarkdownDescription: "The current `.env` file contents as returned by Forge.", Computed: true, Sensitive: true},
		},
	}
}

func (d *siteEnvironmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteEnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteEnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/environment", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteEnvironmentAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site environment", err.Error())
		return
	}

	data.Content = types.StringValue(a.Content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
