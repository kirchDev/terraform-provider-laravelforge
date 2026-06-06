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
	_ datasource.DataSource              = (*monitorDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*monitorDataSource)(nil)
)

// NewMonitorDataSource returns a new laravelforge_monitor data source.
func NewMonitorDataSource() datasource.DataSource {
	return &monitorDataSource{}
}

type monitorDataSource struct {
	client *client.Client
}

// monitorAttributes mirrors the JSON:API "attributes" of a monitor resource.
// threshold is `number` in the OpenAPI spec (percentage value); minutes is a
// nullable integer; state_changed_at is a nullable timestamp.
type monitorAttributes struct {
	Type           string  `json:"type"`
	Operator       string  `json:"operator"`
	Threshold      float64 `json:"threshold"`
	Minutes        *int64  `json:"minutes"`
	Notify         string  `json:"notify"`
	Status         string  `json:"status"`
	State          string  `json:"state"`
	StateChangedAt *string `json:"state_changed_at"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type monitorDataSourceModel struct {
	Organization   types.String  `tfsdk:"organization"`
	ServerID       types.Int64   `tfsdk:"server_id"`
	ID             types.Int64   `tfsdk:"id"`
	Type           types.String  `tfsdk:"type"`
	Operator       types.String  `tfsdk:"operator"`
	Threshold      types.Float64 `tfsdk:"threshold"`
	Minutes        types.Int64   `tfsdk:"minutes"`
	Notify         types.String  `tfsdk:"notify"`
	Status         types.String  `tfsdk:"status"`
	State          types.String  `tfsdk:"state"`
	StateChangedAt types.String  `tfsdk:"state_changed_at"`
	CreatedAt      types.String  `tfsdk:"created_at"`
	UpdatedAt      types.String  `tfsdk:"updated_at"`
}

func (d *monitorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (d *monitorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge server monitor by ID.",
		Attributes: map[string]schema.Attribute{
			"organization":     schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":        schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the monitor belongs to.", Required: true},
			"id":               schema.Int64Attribute{MarkdownDescription: "Numeric ID of the monitor.", Required: true},
			"type":             schema.StringAttribute{MarkdownDescription: "Metric type (`cpu_load`, `disk`, `free_memory`, or `used_memory`).", Computed: true},
			"operator":         schema.StringAttribute{MarkdownDescription: "Comparison operator against the threshold (`gte` or `lte`).", Computed: true},
			"threshold":        schema.Float64Attribute{MarkdownDescription: "Percentage threshold to alert on once breached.", Computed: true},
			"minutes":          schema.Int64Attribute{MarkdownDescription: "Frequency in minutes to evaluate the monitor.", Computed: true},
			"notify":           schema.StringAttribute{MarkdownDescription: "Email address notified when the monitor is in an alert state.", Computed: true},
			"status":           schema.StringAttribute{MarkdownDescription: "Installation status of the monitor (e.g. `installed`).", Computed: true},
			"state":            schema.StringAttribute{MarkdownDescription: "Current state of the monitor (`OK`, `ALERT`, or `UNKNOWN`).", Computed: true},
			"state_changed_at": schema.StringAttribute{MarkdownDescription: "Timestamp the monitor state last changed.", Computed: true},
			"created_at":       schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":       schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *monitorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *monitorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data monitorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single-monitor reads are server-scoped (the resource's links.self), unlike
	// sites which read at the org level.
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/monitors/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a monitorAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge monitor", err.Error())
		return
	}

	data.Type = types.StringValue(a.Type)
	data.Operator = types.StringValue(a.Operator)
	data.Threshold = types.Float64Value(a.Threshold)
	data.Minutes = types.Int64PointerValue(a.Minutes)
	data.Notify = types.StringValue(a.Notify)
	data.Status = types.StringValue(a.Status)
	data.State = types.StringValue(a.State)
	data.StateChangedAt = types.StringPointerValue(a.StateChangedAt)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
