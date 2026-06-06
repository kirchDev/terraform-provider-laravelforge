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
	_ datasource.DataSource              = (*daemonDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*daemonDataSource)(nil)
)

// NewDaemonDataSource returns a new laravelforge_daemon data source.
func NewDaemonDataSource() datasource.DataSource {
	return &daemonDataSource{}
}

type daemonDataSource struct {
	client *client.Client
}

// daemonAttributes mirrors the JSON:API "attributes" of a daemon (background
// process). Verified live 2026-06-06 against
// /api/orgs/{org}/servers/{id}/background-processes (type "backgroundProcesses").
type daemonAttributes struct {
	Command   string `json:"command"`
	User      string `json:"user"`
	Directory string `json:"directory"`
	Processes int64  `json:"processes"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type daemonDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Command      types.String `tfsdk:"command"`
	User         types.String `tfsdk:"user"`
	Directory    types.String `tfsdk:"directory"`
	Processes    types.Int64  `tfsdk:"processes"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (d *daemonDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_daemon"
}

func (d *daemonDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge daemon (background process) by ID on a server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the daemon runs on.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the daemon.", Required: true},
			"command":      schema.StringAttribute{MarkdownDescription: "Command the daemon runs.", Computed: true},
			"user":         schema.StringAttribute{MarkdownDescription: "User account the command runs under.", Computed: true},
			"directory":    schema.StringAttribute{MarkdownDescription: "Working directory the command runs in.", Computed: true},
			"processes":    schema.Int64Attribute{MarkdownDescription: "Number of processes to run.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *daemonDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *daemonDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data daemonDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single-daemon reads are server-scoped (verified: the org-scoped path 404s).
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/background-processes/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a daemonAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge daemon", err.Error())
		return
	}

	data.Command = types.StringValue(a.Command)
	data.User = types.StringValue(a.User)
	data.Directory = types.StringValue(a.Directory)
	data.Processes = types.Int64Value(a.Processes)
	data.Status = types.StringValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
