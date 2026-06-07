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
	_ datasource.DataSource              = (*webhookDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*webhookDataSource)(nil)
)

// NewWebhookDataSource returns a new laravelforge_webhook data source.
func NewWebhookDataSource() datasource.DataSource {
	return &webhookDataSource{}
}

type webhookDataSource struct {
	client *client.Client
}

// webhookAttributes mirrors the JSON:API "attributes" of a site webhook.
type webhookAttributes struct {
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

type webhookDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.Int64  `tfsdk:"id"`
	URL          types.String `tfsdk:"url"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (d *webhookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (d *webhookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge site deployment webhook by ID.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the webhook belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the webhook.", Required: true},
			"url":          schema.StringAttribute{MarkdownDescription: "URL the webhook posts to on deployment.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *webhookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *webhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data webhookDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/webhooks/%d",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a webhookAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge webhook", err.Error())
		return
	}

	data.URL = types.StringValue(a.URL)
	data.CreatedAt = types.StringValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
