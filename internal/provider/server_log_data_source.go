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
	_ datasource.DataSource              = (*serverLogDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverLogDataSource)(nil)
)

// NewServerLogDataSource returns a new laravelforge_server_log data source.
func NewServerLogDataSource() datasource.DataSource {
	return &serverLogDataSource{}
}

type serverLogDataSource struct {
	client *client.Client
}

// serverLogAttributes mirrors the JSON:API "attributes" of a server log resource.
type serverLogAttributes struct {
	Content string `json:"content"`
}

type serverLogDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	Key          types.String `tfsdk:"key"`
	ID           types.String `tfsdk:"id"`
	Content      types.String `tfsdk:"content"`
}

func (d *serverLogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_log"
}

func (d *serverLogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the content of a single Laravel Forge server log by key.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization that owns the server.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the log belongs to.", Required: true},
			"key":          schema.StringAttribute{MarkdownDescription: "Key identifying the server log to fetch.", Required: true},
			"id":           schema.StringAttribute{MarkdownDescription: "Resource-level identifier of the server log.", Computed: true},
			"content":      schema.StringAttribute{MarkdownDescription: "Content of the server log.", Computed: true},
		},
	}
}

func (d *serverLogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverLogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverLogDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/logs/%s", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.Key.ValueString())
	var a serverLogAttributes
	id, err := d.client.Get(ctx, path, &a)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server log", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	data.Content = types.StringValue(a.Content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
