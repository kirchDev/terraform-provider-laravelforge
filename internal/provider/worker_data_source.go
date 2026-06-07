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
	_ datasource.DataSource              = (*workerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workerDataSource)(nil)
)

// NewWorkerDataSource returns a new laravelforge_worker data source.
//
// In the new org-scoped Forge API the legacy site-scoped "queue worker" has
// become the server-scoped "background process" (tag "Background Processes",
// type "backgroundProcesses"). The entity is verified server-scoped against the
// live API (2026-06-06): there is no org-level or site-level read path.
func NewWorkerDataSource() datasource.DataSource {
	return &workerDataSource{}
}

type workerDataSource struct {
	client *client.Client
}

// workerAttributes mirrors the JSON:API "attributes" of a background process
// (read shape). Only scalars returned by the read endpoint are mapped; the
// write-only inputs (name, site_id, startsecs, stopwaitsecs, stopsignal) are
// not part of the read response.
type workerAttributes struct {
	Command   string  `json:"command"`
	User      string  `json:"user"`
	Directory *string `json:"directory"`
	Processes int64   `json:"processes"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type workerDataSourceModel struct {
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

func (d *workerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker"
}

func (d *workerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge worker (server background process) by ID on a server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the worker runs on.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the worker (background process).", Required: true},
			"command":      schema.StringAttribute{MarkdownDescription: "Command the worker runs.", Computed: true},
			"user":         schema.StringAttribute{MarkdownDescription: "System user the worker runs as.", Computed: true},
			"directory":    schema.StringAttribute{MarkdownDescription: "Working directory of the worker.", Computed: true},
			"processes":    schema.Int64Attribute{MarkdownDescription: "Number of processes the worker runs.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *workerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Workers (background processes) are server-scoped; the single-resource read
	// path matches the create/list path (no org-level path exists).
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/background-processes/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a workerAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge worker", err.Error())
		return
	}

	data.Command = types.StringValue(a.Command)
	data.User = types.StringValue(a.User)
	data.Directory = types.StringPointerValue(a.Directory)
	data.Processes = types.Int64Value(a.Processes)
	data.Status = types.StringValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
