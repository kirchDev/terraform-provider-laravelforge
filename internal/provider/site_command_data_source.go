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
	_ datasource.DataSource              = (*siteCommandDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteCommandDataSource)(nil)
)

// NewSiteCommandDataSource returns a new laravelforge_site_command data source.
func NewSiteCommandDataSource() datasource.DataSource {
	return &siteCommandDataSource{}
}

type siteCommandDataSource struct {
	client *client.Client
}

// siteCommandAttributes mirrors the JSON:API "attributes" of a command resource.
type siteCommandAttributes struct {
	Command   string `json:"command"`
	Status    string `json:"status"`
	Duration  string `json:"duration"`
	UserID    int64  `json:"user_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type siteCommandDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.Int64  `tfsdk:"id"`
	Command      types.String `tfsdk:"command"`
	Status       types.String `tfsdk:"status"`
	Duration     types.String `tfsdk:"duration"`
	UserID       types.Int64  `tfsdk:"user_id"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *siteCommandDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_command"
}

func (d *siteCommandDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge site command run by ID, from a site's command run history.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization that owns the site.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server that hosts the site.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the command ran on.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the command run.", Required: true},
			"command":      schema.StringAttribute{MarkdownDescription: "The command that ran.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Status of the command (`waiting`, `running`, `finished`, `timeout`, `failed`).", Computed: true},
			"duration":     schema.StringAttribute{MarkdownDescription: "Duration of the command in human-readable format (e.g. `5m`).", Computed: true},
			"user_id":      schema.Int64Attribute{MarkdownDescription: "ID of the user who initiated the command.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Timestamp the command was created.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Timestamp the command was last updated.", Computed: true},
		},
	}
}

func (d *siteCommandDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteCommandDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteCommandDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/commands/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a siteCommandAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site command", err.Error())
		return
	}

	data.Command = types.StringValue(a.Command)
	data.Status = types.StringValue(a.Status)
	data.Duration = types.StringValue(a.Duration)
	data.UserID = types.Int64Value(a.UserID)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
