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
	_ datasource.DataSource              = (*siteNginxErrorLogDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteNginxErrorLogDataSource)(nil)
)

// NewSiteNginxErrorLogDataSource returns a new laravelforge_site_nginx_error_log data source.
func NewSiteNginxErrorLogDataSource() datasource.DataSource {
	return &siteNginxErrorLogDataSource{}
}

type siteNginxErrorLogDataSource struct {
	client *client.Client
}

// siteNginxErrorLogAttributes mirrors the JSON:API "attributes" of the nginx error log.
type siteNginxErrorLogAttributes struct {
	Content string `json:"content"`
}

type siteNginxErrorLogDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.String `tfsdk:"id"`
	Content      types.String `tfsdk:"content"`
}

func (d *siteNginxErrorLogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_nginx_error_log"
}

func (d *siteNginxErrorLogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the nginx error log content for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site.", Required: true},
			"id":           schema.StringAttribute{MarkdownDescription: "Identifier of the nginx error log resource.", Computed: true},
			"content":      schema.StringAttribute{MarkdownDescription: "Contents of the nginx error log.", Computed: true},
		},
	}
}

func (d *siteNginxErrorLogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteNginxErrorLogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteNginxErrorLogDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/logs/nginx-error", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteNginxErrorLogAttributes
	id, err := d.client.Get(ctx, path, &a)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site nginx error log", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	data.Content = types.StringValue(a.Content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
