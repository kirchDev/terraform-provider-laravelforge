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
	_ datasource.DataSource              = (*siteReverbDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteReverbDataSource)(nil)
)

// NewSiteReverbDataSource returns a new laravelforge_site_reverb data source.
func NewSiteReverbDataSource() datasource.DataSource {
	return &siteReverbDataSource{}
}

type siteReverbDataSource struct {
	client *client.Client
}

type siteReverbDataSourceModel struct {
	Organization    types.String `tfsdk:"organization"`
	ServerID        types.Int64  `tfsdk:"server_id"`
	SiteID          types.Int64  `tfsdk:"site_id"`
	Host            types.String `tfsdk:"host"`
	Port            types.Int64  `tfsdk:"port"`
	Connections     types.Int64  `tfsdk:"connections"`
	Enabled         types.String `tfsdk:"enabled"`
	ReverbInstalled types.Bool   `tfsdk:"reverb_installed"`
}

func (d *siteReverbDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_reverb"
}

func (d *siteReverbDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the Laravel Reverb integration status for a Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization":     schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":        schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":          schema.Int64Attribute{MarkdownDescription: "ID of the site.", Required: true},
			"host":             schema.StringAttribute{MarkdownDescription: "Reverb host.", Computed: true},
			"port":             schema.Int64Attribute{MarkdownDescription: "Reverb port.", Computed: true},
			"connections":      schema.Int64Attribute{MarkdownDescription: "Maximum number of concurrent connections.", Computed: true},
			"enabled":          schema.StringAttribute{MarkdownDescription: "Whether the Reverb integration is enabled, as reported by Forge.", Computed: true},
			"reverb_installed": schema.BoolAttribute{MarkdownDescription: "Whether the Reverb package is installed on the site.", Computed: true},
		},
	}
}

func (d *siteReverbDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteReverbDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteReverbDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/reverb",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteReverbAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge Reverb integration", err.Error())
		return
	}

	data.Enabled = types.StringValue(a.Enabled)
	data.ReverbInstalled = types.BoolValue(a.ReverbInstalled)
	data.Host = types.StringPointerValue(a.Host)
	data.Port = types.Int64PointerValue(a.Port)
	data.Connections = types.Int64PointerValue(a.Connections)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
