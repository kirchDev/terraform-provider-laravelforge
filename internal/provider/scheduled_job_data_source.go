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
	_ datasource.DataSource              = (*scheduledJobDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*scheduledJobDataSource)(nil)
)

// NewScheduledJobDataSource returns a new laravelforge_scheduled_job data source.
func NewScheduledJobDataSource() datasource.DataSource {
	return &scheduledJobDataSource{}
}

type scheduledJobDataSource struct {
	client *client.Client
}

// scheduledJobAttributes mirrors the JSON:API "attributes" of a scheduled job
// (read shape). Verified against the live API 2026-06-06. Note that the read
// "frequency" is capitalized (e.g. "Custom", "Nightly") even though create
// sends it lowercase.
type scheduledJobAttributes struct {
	Name        *string `json:"name"`
	Command     string  `json:"command"`
	Status      string  `json:"status"`
	User        string  `json:"user"`
	Frequency   string  `json:"frequency"`
	Cron        *string `json:"cron"`
	NextRunTime *string `json:"next_run_time"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type scheduledJobDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Command      types.String `tfsdk:"command"`
	Status       types.String `tfsdk:"status"`
	User         types.String `tfsdk:"user"`
	Frequency    types.String `tfsdk:"frequency"`
	Cron         types.String `tfsdk:"cron"`
	NextRunTime  types.String `tfsdk:"next_run_time"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *scheduledJobDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scheduled_job"
}

func (d *scheduledJobDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge scheduled job (cron job) by ID on a server.",
		Attributes: map[string]schema.Attribute{
			"organization":  schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":     schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the job runs on.", Required: true},
			"id":            schema.Int64Attribute{MarkdownDescription: "Numeric ID of the scheduled job.", Required: true},
			"name":          schema.StringAttribute{MarkdownDescription: "Job name (may be null).", Computed: true},
			"command":       schema.StringAttribute{MarkdownDescription: "Command the job executes.", Computed: true},
			"status":        schema.StringAttribute{MarkdownDescription: "Provisioning status (e.g. `installing`, `installed`).", Computed: true},
			"user":          schema.StringAttribute{MarkdownDescription: "System user the command runs as.", Computed: true},
			"frequency":     schema.StringAttribute{MarkdownDescription: "How often the job runs (e.g. `Minutely`, `Nightly`, `Custom`).", Computed: true},
			"cron":          schema.StringAttribute{MarkdownDescription: "Cron expression for the job.", Computed: true},
			"next_run_time": schema.StringAttribute{MarkdownDescription: "Timestamp of the next scheduled run.", Computed: true},
			"created_at":    schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":    schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (d *scheduledJobDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *scheduledJobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data scheduledJobDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single-job reads are server-scoped (per the resource's links.self).
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/scheduled-jobs/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a scheduledJobAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge scheduled job", err.Error())
		return
	}

	data.Name = types.StringPointerValue(a.Name)
	data.Command = types.StringValue(a.Command)
	data.Status = types.StringValue(a.Status)
	data.User = types.StringValue(a.User)
	data.Frequency = types.StringValue(a.Frequency)
	data.Cron = types.StringPointerValue(a.Cron)
	data.NextRunTime = types.StringPointerValue(a.NextRunTime)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
