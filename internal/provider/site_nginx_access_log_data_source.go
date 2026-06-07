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
	_ datasource.DataSource              = (*siteNginxAccessLogDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteNginxAccessLogDataSource)(nil)
)

// NewSiteNginxAccessLogDataSource returns a new laravelforge_site_nginx_access_log
// data source. It reads a site's Nginx access log (singleton; GET show only).
func NewSiteNginxAccessLogDataSource() datasource.DataSource {
	return &siteNginxAccessLogDataSource{}
}

type siteNginxAccessLogDataSource struct {
	client *client.Client
}

// siteNginxAccessLogAttributes mirrors the JSON:API "attributes" of a
// NginxAccessLogResource.
type siteNginxAccessLogAttributes struct {
	Content string `json:"content"`
}

type siteNginxAccessLogDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.String `tfsdk:"id"`
	Content      types.String `tfsdk:"content"`
}

func (d *siteNginxAccessLogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_nginx_access_log"
}

func (d *siteNginxAccessLogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the Nginx access log content for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site runs on.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site.", Required: true},
			"id":           schema.StringAttribute{MarkdownDescription: "Resource identifier of the Nginx access log.", Computed: true},
			"content":      schema.StringAttribute{MarkdownDescription: "Nginx access log content.", Computed: true},
		},
	}
}

func (d *siteNginxAccessLogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteNginxAccessLogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteNginxAccessLogDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/logs/nginx-access",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteNginxAccessLogAttributes
	id, err := d.client.Get(ctx, path, &a)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site Nginx access log", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	data.Content = types.StringValue(a.Content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
