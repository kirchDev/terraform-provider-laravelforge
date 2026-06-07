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
	_ datasource.DataSource              = (*serverBackupInstanceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverBackupInstanceDataSource)(nil)
)

// NewServerBackupInstanceDataSource returns a new laravelforge_server_backup_instance data source.
func NewServerBackupInstanceDataSource() datasource.DataSource {
	return &serverBackupInstanceDataSource{}
}

type serverBackupInstanceDataSource struct {
	client *client.Client
}

// serverBackupInstanceAttributes mirrors the JSON:API "attributes" of a backup
// instance (BackupResource). A backup instance is a read-only artifact of a
// backup configuration.
type serverBackupInstanceAttributes struct {
	Status     string `json:"status"`
	IsPartial  string `json:"is_partial"`
	Size       int64  `json:"size"`
	FinishedAt string `json:"finished_at"`
}

type serverBackupInstanceDataSourceModel struct {
	Organization          types.String `tfsdk:"organization"`
	ServerID              types.Int64  `tfsdk:"server_id"`
	BackupConfigurationID types.Int64  `tfsdk:"backup_configuration_id"`
	ID                    types.Int64  `tfsdk:"id"`
	Status                types.String `tfsdk:"status"`
	IsPartial             types.String `tfsdk:"is_partial"`
	Size                  types.Int64  `tfsdk:"size"`
	FinishedAt            types.String `tfsdk:"finished_at"`
}

func (d *serverBackupInstanceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_backup_instance"
}

func (d *serverBackupInstanceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single backup instance (a completed backup run) of a backup configuration on a server.",
		Attributes: map[string]schema.Attribute{
			"organization":            schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":               schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server.", Required: true},
			"backup_configuration_id": schema.Int64Attribute{MarkdownDescription: "Numeric ID of the backup configuration the instance belongs to.", Required: true},
			"id":                      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the backup instance.", Required: true},
			"status":                  schema.StringAttribute{MarkdownDescription: "Status of the backup run.", Computed: true},
			"is_partial":              schema.StringAttribute{MarkdownDescription: "Whether the backup is partial.", Computed: true},
			"size":                    schema.Int64Attribute{MarkdownDescription: "Size of the backup in bytes.", Computed: true},
			"finished_at":             schema.StringAttribute{MarkdownDescription: "Timestamp the backup run finished.", Computed: true},
		},
	}
}

func (d *serverBackupInstanceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverBackupInstanceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverBackupInstanceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf(
		"/api/orgs/%s/servers/%d/database/backups/%d/instances/%d",
		data.Organization.ValueString(),
		data.ServerID.ValueInt64(),
		data.BackupConfigurationID.ValueInt64(),
		data.ID.ValueInt64(),
	)
	var a serverBackupInstanceAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge backup instance", err.Error())
		return
	}

	data.Status = types.StringValue(a.Status)
	data.IsPartial = types.StringValue(a.IsPartial)
	data.Size = types.Int64Value(a.Size)
	data.FinishedAt = types.StringValue(a.FinishedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
