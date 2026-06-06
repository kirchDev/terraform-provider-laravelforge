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
	_ datasource.DataSource              = (*serverBackgroundProcessDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverBackgroundProcessDataSource)(nil)
)

// NewServerBackgroundProcessDataSource returns a new
// laravelforge_server_background_process data source.
func NewServerBackgroundProcessDataSource() datasource.DataSource {
	return &serverBackgroundProcessDataSource{}
}

type serverBackgroundProcessDataSource struct {
	client *client.Client
}

type serverBackgroundProcessDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Type         types.String `tfsdk:"type"`
	Command      types.String `tfsdk:"command"`
	User         types.String `tfsdk:"user"`
	Directory    types.String `tfsdk:"directory"`
	Processes    types.Int64  `tfsdk:"processes"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (d *serverBackgroundProcessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_background_process"
}

func (d *serverBackgroundProcessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single background process by ID on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the background process runs on.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the background process.", Required: true},
			"type":         schema.StringAttribute{MarkdownDescription: "JSON:API resource type.", Computed: true},
			"command":      schema.StringAttribute{MarkdownDescription: "The command that the background process is running.", Computed: true},
			"user":         schema.StringAttribute{MarkdownDescription: "The user that the background process is running as.", Computed: true},
			"directory":    schema.StringAttribute{MarkdownDescription: "The directory that the background process is running in.", Computed: true},
			"processes":    schema.Int64Attribute{MarkdownDescription: "The number of processes that the background process is running.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "The status of the background process.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *serverBackgroundProcessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverBackgroundProcessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverBackgroundProcessDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/background-processes/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a serverBackgroundProcessAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge background process", err.Error())
		return
	}

	data.Type = types.StringValue("backgroundProcesses")
	data.Command = types.StringValue(a.Command)
	data.User = types.StringValue(a.User)
	data.Directory = types.StringPointerValue(a.Directory)
	data.Processes = types.Int64Value(a.Processes)
	data.Status = types.StringValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
