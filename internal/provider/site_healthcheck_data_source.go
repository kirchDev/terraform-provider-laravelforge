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
	_ datasource.DataSource              = (*siteHealthcheckDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteHealthcheckDataSource)(nil)
)

// NewSiteHealthcheckDataSource returns a new laravelforge_site_healthcheck data source.
func NewSiteHealthcheckDataSource() datasource.DataSource {
	return &siteHealthcheckDataSource{}
}

type siteHealthcheckDataSource struct {
	client *client.Client
}

type siteHealthcheckDataSourceModel struct {
	Organization        types.String `tfsdk:"organization"`
	ServerID            types.Int64  `tfsdk:"server_id"`
	SiteID              types.Int64  `tfsdk:"site_id"`
	HealthcheckEndpoint types.String `tfsdk:"healthcheck_endpoint"`
}

func (d *siteHealthcheckDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_healthcheck"
}

func (d *siteHealthcheckDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the healthcheck endpoint configuration for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization":         schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":            schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":              schema.Int64Attribute{MarkdownDescription: "ID of the site whose healthcheck endpoint is fetched.", Required: true},
			"healthcheck_endpoint": schema.StringAttribute{MarkdownDescription: "The endpoint / URL used to perform healthchecks.", Computed: true},
		},
	}
}

func (d *siteHealthcheckDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteHealthcheckDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteHealthcheckDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/healthcheck", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteHealthcheckAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site healthcheck endpoint", err.Error())
		return
	}

	data.HealthcheckEndpoint = types.StringPointerValue(a.HealthcheckEndpoint)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
