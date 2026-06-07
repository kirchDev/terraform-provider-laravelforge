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
	_ datasource.DataSource              = (*orgSiteLookupDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgSiteLookupDataSource)(nil)
)

// NewOrgSiteLookupDataSource returns a new laravelforge_org_site_lookup data
// source. It looks up a single site org-wide by ID — without needing to know
// which server hosts it — via GET /api/orgs/{organization}/sites/{site}. It
// complements laravelforge_site (server-scoped) and reuses the shared
// siteAttributes read shape from site_resource.go.
func NewOrgSiteLookupDataSource() datasource.DataSource {
	return &orgSiteLookupDataSource{}
}

type orgSiteLookupDataSource struct {
	client *client.Client
}

type orgSiteLookupDataSourceModel struct {
	Organization            types.String `tfsdk:"organization"`
	ID                      types.Int64  `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	PHPVersion              types.String `tfsdk:"php_version"`
	RootDirectory           types.String `tfsdk:"root_directory"`
	WebDirectory            types.String `tfsdk:"web_directory"`
	Status                  types.String `tfsdk:"status"`
	URL                     types.String `tfsdk:"url"`
	ZeroDowntimeDeployments types.Bool   `tfsdk:"zero_downtime_deployments"`
	CreatedAt               types.String `tfsdk:"created_at"`
}

func (d *orgSiteLookupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_site_lookup"
}

func (d *orgSiteLookupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single Laravel Forge site org-wide by ID, across all servers, without needing to know which server hosts it. Complements `laravelforge_site` (server-scoped).",
		Attributes: map[string]schema.Attribute{
			"organization":              schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"id":                        schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site.", Required: true},
			"name":                      schema.StringAttribute{MarkdownDescription: "Site domain name.", Computed: true},
			"php_version":               schema.StringAttribute{MarkdownDescription: "PHP version key (e.g. `php82`).", Computed: true},
			"root_directory":            schema.StringAttribute{MarkdownDescription: "Project root directory.", Computed: true},
			"web_directory":             schema.StringAttribute{MarkdownDescription: "Web directory served by nginx.", Computed: true},
			"status":                    schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"url":                       schema.StringAttribute{MarkdownDescription: "Site URL.", Computed: true},
			"zero_downtime_deployments": schema.BoolAttribute{MarkdownDescription: "Whether zero-downtime deployments are enabled.", Computed: true},
			"created_at":                schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *orgSiteLookupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgSiteLookupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgSiteLookupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Org-wide single-site show: no server_id required.
	path := fmt.Sprintf("/api/orgs/%s/sites/%d", data.Organization.ValueString(), data.ID.ValueInt64())
	var a siteAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.PHPVersion = types.StringPointerValue(a.PHPVersion)
	data.RootDirectory = types.StringPointerValue(a.RootDirectory)
	data.WebDirectory = types.StringPointerValue(a.WebDirectory)
	data.Status = types.StringValue(a.Status)
	data.URL = types.StringPointerValue(a.URL)
	data.ZeroDowntimeDeployments = types.BoolValue(a.ZeroDowntimeDeployments)
	data.CreatedAt = types.StringValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
