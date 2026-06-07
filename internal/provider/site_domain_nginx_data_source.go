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
	_ datasource.DataSource              = (*siteDomainNginxDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDomainNginxDataSource)(nil)
)

// NewSiteDomainNginxDataSource returns a new laravelforge_site_domain_nginx data source.
func NewSiteDomainNginxDataSource() datasource.DataSource {
	return &siteDomainNginxDataSource{}
}

type siteDomainNginxDataSource struct {
	client *client.Client
}

type siteDomainNginxDataSourceModel struct {
	Organization   types.String `tfsdk:"organization"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	SiteID         types.Int64  `tfsdk:"site_id"`
	DomainRecordID types.Int64  `tfsdk:"domain_record_id"`
	Content        types.String `tfsdk:"content"`
}

func (d *siteDomainNginxDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_domain_nginx"
}

func (d *siteDomainNginxDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the per-domain nginx configuration block for a Laravel Forge site domain record.",
		Attributes: map[string]schema.Attribute{
			"organization":     schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":        schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":          schema.Int64Attribute{MarkdownDescription: "ID of the site the domain record belongs to.", Required: true},
			"domain_record_id": schema.Int64Attribute{MarkdownDescription: "ID of the domain record whose nginx config this reads.", Required: true},
			"content":          schema.StringAttribute{MarkdownDescription: "Current nginx configuration content as reported by Forge.", Computed: true},
		},
	}
}

func (d *siteDomainNginxDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDomainNginxDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDomainNginxDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf(
		"/api/orgs/%s/servers/%d/sites/%d/domains/%d/nginx",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.DomainRecordID.ValueInt64(),
	)
	var a siteDomainNginxAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge domain nginx config", err.Error())
		return
	}

	data.Content = types.StringPointerValue(a.Content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
