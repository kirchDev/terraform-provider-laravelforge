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
	_ datasource.DataSource              = (*siteDeploymentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDeploymentDataSource)(nil)
)

// NewSiteDeploymentDataSource returns a new laravelforge_site_deployment data source.
func NewSiteDeploymentDataSource() datasource.DataSource {
	return &siteDeploymentDataSource{}
}

type siteDeploymentDataSource struct {
	client *client.Client
}

// siteDeploymentAttributes mirrors the JSON:API "attributes" of a deployment
// resource. The nested "commit" object is intentionally omitted (scalars only).
type siteDeploymentAttributes struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type siteDeploymentDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.Int64  `tfsdk:"id"`
	Type         types.String `tfsdk:"type"`
	Status       types.String `tfsdk:"status"`
	StartedAt    types.String `tfsdk:"started_at"`
	EndedAt      types.String `tfsdk:"ended_at"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *siteDeploymentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deployment"
}

func (d *siteDeploymentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge site deployment (deployment history) by ID.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization that owns the site.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site runs on.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the deployment belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the deployment.", Required: true},
			"type":         schema.StringAttribute{MarkdownDescription: "Deployment type.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Deployment status (e.g. `pending`, `deploying`, `finished`, `failed`, `cancelled`).", Computed: true},
			"started_at":   schema.StringAttribute{MarkdownDescription: "Timestamp the deployment started.", Computed: true},
			"ended_at":     schema.StringAttribute{MarkdownDescription: "Timestamp the deployment ended.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (d *siteDeploymentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDeploymentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDeploymentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deployments/%d",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a siteDeploymentAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site deployment", err.Error())
		return
	}

	data.Type = types.StringValue(a.Type)
	data.Status = types.StringValue(a.Status)
	data.StartedAt = types.StringValue(a.StartedAt)
	data.EndedAt = types.StringValue(a.EndedAt)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
