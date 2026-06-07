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
	_ datasource.DataSource              = (*siteInertiaDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteInertiaDataSource)(nil)
)

// NewSiteInertiaDataSource returns a new laravelforge_site_inertia data source.
func NewSiteInertiaDataSource() datasource.DataSource {
	return &siteInertiaDataSource{}
}

type siteInertiaDataSource struct {
	client *client.Client
}

type siteInertiaDataSourceModel struct {
	Organization     types.String `tfsdk:"organization"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	SiteID           types.Int64  `tfsdk:"site_id"`
	Enabled          types.String `tfsdk:"enabled"`
	InertiaInstalled types.Bool   `tfsdk:"inertia_installed"`
}

func (d *siteInertiaDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_inertia"
}

func (d *siteInertiaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the Inertia SSR integration status for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization":      schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":         schema.Int64Attribute{MarkdownDescription: "ID of the server that hosts the site.", Required: true},
			"site_id":           schema.Int64Attribute{MarkdownDescription: "ID of the site.", Required: true},
			"enabled":           schema.StringAttribute{MarkdownDescription: "Whether the Inertia integration is enabled.", Computed: true},
			"inertia_installed": schema.BoolAttribute{MarkdownDescription: "Whether Inertia is installed for the site.", Computed: true},
		},
	}
}

func (d *siteInertiaDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteInertiaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteInertiaDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/inertia",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteInertiaAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge Inertia integration", err.Error())
		return
	}

	data.Enabled = types.StringValue(a.Enabled)
	data.InertiaInstalled = types.BoolValue(a.InertiaInstalled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
