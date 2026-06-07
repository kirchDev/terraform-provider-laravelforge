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
	_ datasource.DataSource              = (*siteDeployHookDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDeployHookDataSource)(nil)
)

// NewSiteDeployHookDataSource returns a new laravelforge_site_deploy_hook data source.
func NewSiteDeployHookDataSource() datasource.DataSource {
	return &siteDeployHookDataSource{}
}

type siteDeployHookDataSource struct {
	client *client.Client
}

type siteDeployHookDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	URL          types.String `tfsdk:"url"`
}

func (d *siteDeployHookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deploy_hook"
}

func (d *siteDeployHookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the deployment-trigger (deploy-hook) URL for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "ID of the site whose deploy hook is fetched.", Required: true},
			"url":          schema.StringAttribute{MarkdownDescription: "The deployment-trigger URL for the site.", Computed: true},
		},
	}
}

func (d *siteDeployHookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDeployHookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDeployHookDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deployments/deploy-hook",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteDeployHookAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site deploy hook", err.Error())
		return
	}

	data.URL = types.StringValue(a.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
