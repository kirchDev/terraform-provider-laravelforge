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
	_ datasource.DataSource              = (*siteApplicationLogDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteApplicationLogDataSource)(nil)
)

// NewSiteApplicationLogDataSource returns a new laravelforge_site_application_log
// data source.
func NewSiteApplicationLogDataSource() datasource.DataSource {
	return &siteApplicationLogDataSource{}
}

type siteApplicationLogDataSource struct {
	client *client.Client
}

// siteApplicationLogAttributes mirrors the JSON:API "attributes" of an
// application-log resource.
type siteApplicationLogAttributes struct {
	Content string `json:"content"`
}

type siteApplicationLogDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Content      types.String `tfsdk:"content"`
}

func (d *siteApplicationLogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_application_log"
}

func (d *siteApplicationLogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the application log content for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site.", Required: true},
			"content":      schema.StringAttribute{MarkdownDescription: "The content of the application log.", Computed: true},
		},
	}
}

func (d *siteApplicationLogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteApplicationLogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteApplicationLogDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/logs/application", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteApplicationLogAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site application log", err.Error())
		return
	}

	data.Content = types.StringValue(a.Content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
