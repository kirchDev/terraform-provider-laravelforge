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
	_ datasource.DataSource              = (*providerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*providerDataSource)(nil)
)

// NewProviderDataSource returns a new laravelforge_provider data source.
func NewProviderDataSource() datasource.DataSource {
	return &providerDataSource{}
}

type providerDataSource struct {
	client *client.Client
}

// providerAttributes mirrors the JSON:API "attributes" of a server-provider
// catalog resource (read at /api/providers/{provider}). Providers are global
// (Hetzner, DigitalOcean, AWS, ...), not scoped to an organization.
type providerAttributes struct {
	Name              string  `json:"name"`
	Slug              string  `json:"slug"`
	SimpleName        *string `json:"simple_name"`
	Currency          string  `json:"currency"`
	CurrencySymbol    string  `json:"currency_symbol"`
	DefaultSizeCode   *string `json:"default_size_code"`
	DefaultRegionCode *string `json:"default_region_code"`
}

type providerDataSourceModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Slug              types.String `tfsdk:"slug"`
	SimpleName        types.String `tfsdk:"simple_name"`
	Currency          types.String `tfsdk:"currency"`
	CurrencySymbol    types.String `tfsdk:"currency_symbol"`
	DefaultSizeCode   types.String `tfsdk:"default_size_code"`
	DefaultRegionCode types.String `tfsdk:"default_region_code"`
}

func (d *providerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider"
}

func (d *providerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge server provider (e.g. DigitalOcean, AWS, Hetzner) from the global catalog by ID. Providers are global, not scoped to an organization.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server provider.", Required: true},
			"name":                schema.StringAttribute{MarkdownDescription: "Human-readable provider name.", Computed: true},
			"slug":                schema.StringAttribute{MarkdownDescription: "Provider slug used when provisioning (e.g. `ocean2`, `aws`).", Computed: true},
			"simple_name":         schema.StringAttribute{MarkdownDescription: "Short display name for the provider, if exposed.", Computed: true},
			"currency":            schema.StringAttribute{MarkdownDescription: "Currency code the provider bills in (e.g. `USD`).", Computed: true},
			"currency_symbol":     schema.StringAttribute{MarkdownDescription: "Currency symbol for the billing currency (e.g. `$`).", Computed: true},
			"default_size_code":   schema.StringAttribute{MarkdownDescription: "Default server size code for the provider, if any.", Computed: true},
			"default_region_code": schema.StringAttribute{MarkdownDescription: "Default region code for the provider, if any.", Computed: true},
		},
	}
}

func (d *providerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *providerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data providerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Providers are a global catalog, not org-scoped: /api/providers/{provider}.
	path := fmt.Sprintf("/api/providers/%d", data.ID.ValueInt64())
	var a providerAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge provider", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Slug = types.StringValue(a.Slug)
	data.SimpleName = types.StringPointerValue(a.SimpleName)
	data.Currency = types.StringValue(a.Currency)
	data.CurrencySymbol = types.StringValue(a.CurrencySymbol)
	data.DefaultSizeCode = types.StringPointerValue(a.DefaultSizeCode)
	data.DefaultRegionCode = types.StringPointerValue(a.DefaultRegionCode)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
