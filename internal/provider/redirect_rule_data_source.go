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
	_ datasource.DataSource              = (*redirectRuleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*redirectRuleDataSource)(nil)
)

// NewRedirectRuleDataSource returns a new laravelforge_redirect_rule data source.
func NewRedirectRuleDataSource() datasource.DataSource {
	return &redirectRuleDataSource{}
}

type redirectRuleDataSource struct {
	client *client.Client
}

// redirectRuleAttributes mirrors the JSON:API "attributes" of a redirect rule.
type redirectRuleAttributes struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type redirectRuleDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.Int64  `tfsdk:"id"`
	From         types.String `tfsdk:"from"`
	To           types.String `tfsdk:"to"`
	Type         types.String `tfsdk:"type"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *redirectRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_redirect_rule"
}

func (d *redirectRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single redirect rule by ID on a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the redirect rule belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the redirect rule.", Required: true},
			"from":         schema.StringAttribute{MarkdownDescription: "Source URL path.", Computed: true},
			"to":           schema.StringAttribute{MarkdownDescription: "Destination URL path.", Computed: true},
			"type":         schema.StringAttribute{MarkdownDescription: "Redirect type (`redirect` or `permanent`).", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Provisioning status (`installing`, `installed`, or `removing`).", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *redirectRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *redirectRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data redirectRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Redirect rules are site-scoped on both list and single read (links.self).
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/redirect-rules/%d",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a redirectRuleAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge redirect rule", err.Error())
		return
	}

	data.From = types.StringValue(a.From)
	data.To = types.StringValue(a.To)
	data.Type = types.StringValue(a.Type)
	data.Status = types.StringValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
