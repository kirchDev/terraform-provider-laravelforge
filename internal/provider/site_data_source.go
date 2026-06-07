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
	_ datasource.DataSource              = (*siteDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDataSource)(nil)
)

// NewSiteDataSource returns a new laravelforge_site data source.
func NewSiteDataSource() datasource.DataSource {
	return &siteDataSource{}
}

type siteDataSource struct {
	client *client.Client
}

type siteDataSourceModel struct {
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

func (d *siteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (d *siteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge site by ID on a server.",
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

func (d *siteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single-site reads are org-level (the resource's links.self), even though
	// create/update/delete are server-scoped.
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
