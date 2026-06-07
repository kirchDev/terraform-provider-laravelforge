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
	_ datasource.DataSource              = (*siteDeployScriptDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDeployScriptDataSource)(nil)
)

// NewSiteDeployScriptDataSource returns a new laravelforge_site_deploy_script data source.
func NewSiteDeployScriptDataSource() datasource.DataSource {
	return &siteDeployScriptDataSource{}
}

type siteDeployScriptDataSource struct {
	client *client.Client
}

type siteDeployScriptDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Content      types.String `tfsdk:"content"`
	AutoSource   types.Bool   `tfsdk:"auto_source"`
}

func (d *siteDeployScriptDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deploy_script"
}

func (d *siteDeployScriptDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the deployment script of a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "ID of the site whose deployment script is fetched.", Required: true},
			"content":      schema.StringAttribute{MarkdownDescription: "The content of the deployment script.", Computed: true},
			"auto_source":  schema.BoolAttribute{MarkdownDescription: "Make `.env` variables available to the deployment script.", Computed: true},
		},
	}
}

func (d *siteDeployScriptDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDeployScriptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDeployScriptDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deployments/script",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteDeployScriptAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge deployment script", err.Error())
		return
	}

	data.Content = types.StringPointerValue(a.Content)
	data.AutoSource = types.BoolValue(a.AutoSource)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
