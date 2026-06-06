package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Data-source pattern, following server_data_source.go. ---
//
// Reads a single database backup configuration by ID on a server. The read is
// server-scoped: GET /api/orgs/{org}/servers/{server}/database/backups/{id}.

var (
	_ datasource.DataSource              = (*serverBackupConfigurationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverBackupConfigurationDataSource)(nil)
)

// NewServerBackupConfigurationDataSource returns a new
// laravelforge_server_backup_configuration data source.
func NewServerBackupConfigurationDataSource() datasource.DataSource {
	return &serverBackupConfigurationDataSource{}
}

type serverBackupConfigurationDataSource struct {
	client *client.Client
}

type serverBackupConfigurationDataSourceModel struct {
	Organization        types.String `tfsdk:"organization"`
	ServerID            types.Int64  `tfsdk:"server_id"`
	ID                  types.Int64  `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	StorageProviderID   types.Int64  `tfsdk:"storage_provider_id"`
	CloudProvider       types.String `tfsdk:"cloud_provider"`
	Bucket              types.String `tfsdk:"bucket"`
	Directory           types.String `tfsdk:"directory"`
	Schedule            types.String `tfsdk:"schedule"`
	DisplayableSchedule types.String `tfsdk:"displayable_schedule"`
	NextRunTime         types.String `tfsdk:"next_run_time"`
	Status              types.String `tfsdk:"status"`
	DayOfWeek           types.Int64  `tfsdk:"day_of_week"`
	Time                types.String `tfsdk:"time"`
	CronSchedule        types.String `tfsdk:"cron_schedule"`
	Retention           types.Int64  `tfsdk:"retention"`
	NotifyEmail         types.String `tfsdk:"notify_email"`
}

func (d *serverBackupConfigurationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_backup_configuration"
}

func (d *serverBackupConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single database backup configuration by ID on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization":         schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":            schema.Int64Attribute{MarkdownDescription: "ID of the server the backup configuration belongs to.", Required: true},
			"id":                   schema.Int64Attribute{MarkdownDescription: "Numeric ID of the backup configuration.", Required: true},
			"name":                 schema.StringAttribute{MarkdownDescription: "Name of the backup configuration.", Computed: true},
			"storage_provider_id":  schema.Int64Attribute{MarkdownDescription: "ID of the storage provider (credential) the backups are written to.", Computed: true},
			"cloud_provider":       schema.StringAttribute{MarkdownDescription: "Underlying storage provider (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
			"bucket":               schema.StringAttribute{MarkdownDescription: "Storage bucket the backups are written to.", Computed: true},
			"directory":            schema.StringAttribute{MarkdownDescription: "Directory within the bucket.", Computed: true},
			"schedule":             schema.StringAttribute{MarkdownDescription: "Resolved cron schedule.", Computed: true},
			"displayable_schedule": schema.StringAttribute{MarkdownDescription: "Human-readable schedule.", Computed: true},
			"next_run_time":        schema.StringAttribute{MarkdownDescription: "Timestamp of the next scheduled run.", Computed: true},
			"status":               schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"day_of_week":          schema.Int64Attribute{MarkdownDescription: "Day of week the backup runs (`0`-`6`), if weekly.", Computed: true},
			"time":                 schema.StringAttribute{MarkdownDescription: "Time of day the backup runs (e.g. `03:00`).", Computed: true},
			"cron_schedule":        schema.StringAttribute{MarkdownDescription: "Custom cron schedule, if `frequency` is `custom`.", Computed: true},
			"retention":            schema.Int64Attribute{MarkdownDescription: "Number of backups to retain.", Computed: true},
			"notify_email":         schema.StringAttribute{MarkdownDescription: "Email address notified on backup events.", Computed: true},
		},
	}
}

func (d *serverBackupConfigurationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverBackupConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverBackupConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/database/backups/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a serverBackupConfigurationAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge backup configuration", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.StorageProviderID = types.Int64PointerValue(a.StorageProviderID)
	data.CloudProvider = types.StringValue(a.Provider)
	data.Bucket = types.StringPointerValue(a.Bucket)
	data.Directory = types.StringValue(a.Directory)
	data.Schedule = types.StringValue(a.Schedule)
	data.DisplayableSchedule = types.StringValue(a.DisplayableSchedule)
	data.NextRunTime = types.StringValue(a.NextRunTime)
	data.Status = types.StringValue(a.Status)
	data.DayOfWeek = types.Int64PointerValue(a.DayOfWeek)
	data.Time = types.StringPointerValue(a.Time)
	data.CronSchedule = types.StringPointerValue(a.CronSchedule)
	data.Retention = types.Int64Value(a.Retention)
	data.NotifyEmail = types.StringPointerValue(a.NotifyEmail)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
