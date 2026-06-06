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
	_ datasource.DataSource              = (*siteDeploymentStatusDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDeploymentStatusDataSource)(nil)
)

// NewSiteDeploymentStatusDataSource returns a new laravelforge_site_deployment_status data source.
func NewSiteDeploymentStatusDataSource() datasource.DataSource {
	return &siteDeploymentStatusDataSource{}
}

type siteDeploymentStatusDataSource struct {
	client *client.Client
}

// siteDeploymentStatusAttributes mirrors the JSON:API "attributes" of a
// deployment-status (singleton) resource.
type siteDeploymentStatusAttributes struct {
	Status    *string `json:"status"`
	StartedAt *string `json:"started_at"`
}

type siteDeploymentStatusDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Status       types.String `tfsdk:"status"`
	StartedAt    types.String `tfsdk:"started_at"`
}

func (d *siteDeploymentStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deployment_status"
}

func (d *siteDeploymentStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the current deployment status of a Laravel Forge site (read-only singleton).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site runs on.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site.", Required: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Current deployment status (one of `cancelled`, `deploying`, `failed`, `failed-build`, `finished`, `pending`, `queued`); null when no deployment has run.", Computed: true},
			"started_at":   schema.StringAttribute{MarkdownDescription: "Timestamp the current deployment started.", Computed: true},
		},
	}
}

func (d *siteDeploymentStatusDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDeploymentStatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDeploymentStatusDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deployments/status",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteDeploymentStatusAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge deployment status", err.Error())
		return
	}

	data.Status = types.StringPointerValue(a.Status)
	data.StartedAt = types.StringPointerValue(a.StartedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
