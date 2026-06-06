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
	_ datasource.DataSource              = (*siteLaravelMaintenanceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteLaravelMaintenanceDataSource)(nil)
)

// NewSiteLaravelMaintenanceDataSource returns a new laravelforge_site_laravel_maintenance data source.
func NewSiteLaravelMaintenanceDataSource() datasource.DataSource {
	return &siteLaravelMaintenanceDataSource{}
}

type siteLaravelMaintenanceDataSource struct {
	client *client.Client
}

type siteLaravelMaintenanceDataSourceModel struct {
	Organization     types.String `tfsdk:"organization"`
	ServerID         types.Int64  `tfsdk:"server_id"`
	SiteID           types.Int64  `tfsdk:"site_id"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Status           types.String `tfsdk:"status"`
	LaravelInstalled types.Bool   `tfsdk:"laravel_installed"`
}

func (d *siteLaravelMaintenanceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_laravel_maintenance"
}

func (d *siteLaravelMaintenanceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the Laravel maintenance mode integration status for a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization":      schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":         schema.Int64Attribute{MarkdownDescription: "ID of the server that owns the site.", Required: true},
			"site_id":           schema.Int64Attribute{MarkdownDescription: "ID of the site.", Required: true},
			"enabled":           schema.BoolAttribute{MarkdownDescription: "Whether the maintenance mode integration is enabled.", Computed: true},
			"status":            schema.StringAttribute{MarkdownDescription: "The status of the maintenance mode integration (`enabling` or `disabling`).", Computed: true},
			"laravel_installed": schema.BoolAttribute{MarkdownDescription: "Whether Laravel is installed on the site.", Computed: true},
		},
	}
}

func (d *siteLaravelMaintenanceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteLaravelMaintenanceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteLaravelMaintenanceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/integrations/laravel-maintenance",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64())
	var a siteLaravelMaintenanceAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Laravel maintenance mode", err.Error())
		return
	}

	data.Enabled = types.BoolValue(a.Enabled)
	data.Status = types.StringPointerValue(a.Status)
	data.LaravelInstalled = types.BoolValue(a.LaravelInstalled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
