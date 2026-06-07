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
	_ datasource.DataSource              = (*sitePulseDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sitePulseDataSource)(nil)
)

// NewSitePulseDataSource returns a new laravelforge_site_pulse data source.
func NewSitePulseDataSource() datasource.DataSource {
	return &sitePulseDataSource{}
}

type sitePulseDataSource struct {
	client *client.Client
}

type sitePulseDataSourceModel struct {
	Organization   types.String `tfsdk:"organization"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	SiteID         types.Int64  `tfsdk:"site_id"`
	Enabled        types.String `tfsdk:"enabled"`
	PulseInstalled types.Bool   `tfsdk:"pulse_installed"`
}

func (d *sitePulseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_pulse"
}

func (d *sitePulseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the Laravel Pulse integration status for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization":    schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":       schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":         schema.Int64Attribute{MarkdownDescription: "ID of the site.", Required: true},
			"enabled":         schema.StringAttribute{MarkdownDescription: "Whether the Pulse integration is enabled.", Computed: true},
			"pulse_installed": schema.BoolAttribute{MarkdownDescription: "Whether Laravel Pulse is installed on the site.", Computed: true},
		},
	}
}

func (d *sitePulseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sitePulseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data sitePulseDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/pulse",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a sitePulseAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Pulse integration", err.Error())
		return
	}

	data.Enabled = types.StringValue(a.Enabled)
	data.PulseInstalled = types.BoolValue(a.PulseInstalled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
