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
	_ datasource.DataSource              = (*siteHeartbeatDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteHeartbeatDataSource)(nil)
)

// NewSiteHeartbeatDataSource returns a new laravelforge_site_heartbeat data source.
func NewSiteHeartbeatDataSource() datasource.DataSource {
	return &siteHeartbeatDataSource{}
}

type siteHeartbeatDataSource struct {
	client *client.Client
}

type siteHeartbeatDataSourceModel struct {
	Organization    types.String `tfsdk:"organization"`
	ServerID        types.Int64  `tfsdk:"server_id"`
	SiteID          types.Int64  `tfsdk:"site_id"`
	ID              types.Int64  `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Status          types.String `tfsdk:"status"`
	GracePeriod     types.Int64  `tfsdk:"grace_period"`
	Frequency       types.Int64  `tfsdk:"frequency"`
	CustomFrequency types.String `tfsdk:"custom_frequency"`
	PingURL         types.String `tfsdk:"ping_url"`
}

func (d *siteHeartbeatDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_heartbeat"
}

func (d *siteHeartbeatDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single heartbeat monitor on a Laravel Forge site by ID.",
		Attributes: map[string]schema.Attribute{
			"organization":     schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":        schema.Int64Attribute{MarkdownDescription: "ID of the server that owns the site.", Required: true},
			"site_id":          schema.Int64Attribute{MarkdownDescription: "ID of the site the heartbeat belongs to.", Required: true},
			"id":               schema.Int64Attribute{MarkdownDescription: "Numeric ID of the heartbeat.", Required: true},
			"name":             schema.StringAttribute{MarkdownDescription: "The name of the heartbeat.", Computed: true},
			"grace_period":     schema.Int64Attribute{MarkdownDescription: "The duration (in minutes) after which a heartbeat is considered missing.", Computed: true},
			"frequency":        schema.Int64Attribute{MarkdownDescription: "The interval (in minutes) at which the client is expected to send a ping.", Computed: true},
			"custom_frequency": schema.StringAttribute{MarkdownDescription: "A cron expression representing the custom frequency, used when `frequency` is -1.", Computed: true},
			"status":           schema.StringAttribute{MarkdownDescription: "Current heartbeat status (`pending`, `beating`, or `missing`).", Computed: true},
			"ping_url":         schema.StringAttribute{MarkdownDescription: "The URL the client pings to report a heartbeat.", Computed: true},
		},
	}
}

func (d *siteHeartbeatDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteHeartbeatDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteHeartbeatDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/heartbeats/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a siteHeartbeatAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge heartbeat", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Status = types.StringPointerValue(a.Status)
	data.GracePeriod = types.Int64Value(a.GracePeriod)
	data.Frequency = types.Int64Value(a.Frequency)
	data.CustomFrequency = types.StringPointerValue(a.CustomFrequency)
	data.PingURL = types.StringPointerValue(a.PingURL)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
