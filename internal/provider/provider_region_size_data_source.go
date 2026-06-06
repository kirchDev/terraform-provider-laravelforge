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
	_ datasource.DataSource              = (*providerRegionSizeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*providerRegionSizeDataSource)(nil)
)

// NewProviderRegionSizeDataSource returns a new laravelforge_provider_region_size data source.
func NewProviderRegionSizeDataSource() datasource.DataSource {
	return &providerRegionSizeDataSource{}
}

type providerRegionSizeDataSource struct {
	client *client.Client
}

// providerRegionSizeAttributes mirrors the JSON:API "attributes" of a provider
// region size resource (read at
// /api/providers/{provider}/regions/{providerRegion}/sizes/{providerSize}).
type providerRegionSizeAttributes struct {
	Name string `json:"name"`
}

type providerRegionSizeDataSourceModel struct {
	ProviderID types.Int64  `tfsdk:"provider_id"`
	RegionID   types.Int64  `tfsdk:"region_id"`
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
}

func (d *providerRegionSizeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_region_size"
}

func (d *providerRegionSizeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge provider size available within a specific provider region. Some providers gate which sizes are offered per-region, so this is the region-scoped subset (separate from `laravelforge_provider_size`). Scoped to a server provider and region, not to an organization.",
		Attributes: map[string]schema.Attribute{
			"provider_id": schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server provider that offers the size (Forge API `provider` path segment; `provider` is reserved in HCL).", Required: true},
			"region_id":   schema.Int64Attribute{MarkdownDescription: "Numeric ID of the provider region the size is available in.", Required: true},
			"id":          schema.StringAttribute{MarkdownDescription: "Forge ID of the provider size.", Required: true},
			"name":        schema.StringAttribute{MarkdownDescription: "Human-readable name of the size.", Computed: true},
		},
	}
}

func (d *providerRegionSizeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *providerRegionSizeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data providerRegionSizeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Provider region sizes are provider+region-scoped (the resource's
	// links.self), not org-scoped:
	// /api/providers/{provider}/regions/{providerRegion}/sizes/{providerSize}.
	path := fmt.Sprintf("/api/providers/%d/regions/%d/sizes/%s", data.ProviderID.ValueInt64(), data.RegionID.ValueInt64(), data.ID.ValueString())
	var a providerRegionSizeAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge provider region size", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
