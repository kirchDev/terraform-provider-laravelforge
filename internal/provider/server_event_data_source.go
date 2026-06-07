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
	_ datasource.DataSource              = (*serverEventDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverEventDataSource)(nil)
)

// NewServerEventDataSource returns a new laravelforge_server_event data source.
func NewServerEventDataSource() datasource.DataSource {
	return &serverEventDataSource{}
}

type serverEventDataSource struct {
	client *client.Client
}

// serverEventAttributes mirrors the JSON:API "attributes" of an event resource
// (EventResource). Object/array attributes (relationships.initiator, links) are
// skipped this pass.
type serverEventAttributes struct {
	Description string  `json:"description"`
	RanAs       *string `json:"ran_as"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type serverEventDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.String `tfsdk:"id"`
	Description  types.String `tfsdk:"description"`
	RanAs        types.String `tfsdk:"ran_as"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *serverEventDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_event"
}

func (d *serverEventDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge server provisioning/operation event by ID on a server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server that the event belongs to.", Required: true},
			"id":           schema.StringAttribute{MarkdownDescription: "ID of the event.", Required: true},
			"description":  schema.StringAttribute{MarkdownDescription: "Description of the event.", Computed: true},
			"ran_as":       schema.StringAttribute{MarkdownDescription: "Server user that the event was run as.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last-updated timestamp.", Computed: true},
		},
	}
}

func (d *serverEventDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverEventDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverEventDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/events/%s", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueString())
	var a serverEventAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server event", err.Error())
		return
	}

	data.Description = types.StringValue(a.Description)
	data.RanAs = types.StringPointerValue(a.RanAs)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
