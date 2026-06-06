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
	_ datasource.DataSource              = (*siteDomainDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDomainDataSource)(nil)
)

// NewSiteDomainDataSource returns a new laravelforge_site_domain data source.
func NewSiteDomainDataSource() datasource.DataSource {
	return &siteDomainDataSource{}
}

type siteDomainDataSource struct {
	client *client.Client
}

type siteDomainDataSourceModel struct {
	Organization            types.String `tfsdk:"organization"`
	ServerID                types.Int64  `tfsdk:"server_id"`
	SiteID                  types.Int64  `tfsdk:"site_id"`
	ID                      types.Int64  `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Type                    types.String `tfsdk:"type"`
	Status                  types.String `tfsdk:"status"`
	WwwRedirectType         types.String `tfsdk:"www_redirect_type"`
	AllowWildcardSubdomains types.Bool   `tfsdk:"allow_wildcard_subdomains"`
	CreatedAt               types.String `tfsdk:"created_at"`
	UpdatedAt               types.String `tfsdk:"updated_at"`
}

func (d *siteDomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_domain"
}

func (d *siteDomainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single domain record on a Laravel Forge site by ID.",
		Attributes: map[string]schema.Attribute{
			"organization":              schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":                 schema.Int64Attribute{MarkdownDescription: "ID of the server hosting the site.", Required: true},
			"site_id":                   schema.Int64Attribute{MarkdownDescription: "ID of the site the domain belongs to.", Required: true},
			"id":                        schema.Int64Attribute{MarkdownDescription: "Numeric ID of the domain record.", Required: true},
			"name":                      schema.StringAttribute{MarkdownDescription: "The name of the domain.", Computed: true},
			"type":                      schema.StringAttribute{MarkdownDescription: "The type of domain (`primary` or `alias`).", Computed: true},
			"status":                    schema.StringAttribute{MarkdownDescription: "The status of the domain.", Computed: true},
			"www_redirect_type":         schema.StringAttribute{MarkdownDescription: "Type of `www` redirection (`from-www`, `to-www`, or `none`).", Computed: true},
			"allow_wildcard_subdomains": schema.BoolAttribute{MarkdownDescription: "Whether the domain allows wildcard subdomains.", Computed: true},
			"created_at":                schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":                schema.StringAttribute{MarkdownDescription: "Last-updated timestamp.", Computed: true},
		},
	}
}

func (d *siteDomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDomainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/domains/%d",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a siteDomainAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site domain", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Type = types.StringValue(a.Type)
	data.Status = types.StringValue(a.Status)
	data.WwwRedirectType = types.StringValue(a.WwwRedirectType)
	data.AllowWildcardSubdomains = types.BoolValue(a.AllowWildcardSubdomains)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
