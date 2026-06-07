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
	_ datasource.DataSource              = (*regionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*regionDataSource)(nil)
)

// NewRegionDataSource returns a new laravelforge_region data source.
func NewRegionDataSource() datasource.DataSource {
	return &regionDataSource{}
}

type regionDataSource struct {
	client *client.Client
}

// regionAttributes mirrors the JSON:API "attributes" of a provider region
// resource (read at /api/providers/{provider}/regions/{region}).
type regionAttributes struct {
	Name          string  `json:"name"`
	Code          string  `json:"code"`
	AlternateCode *string `json:"alternate_code"`
}

type regionDataSourceModel struct {
	ProviderID    types.Int64  `tfsdk:"provider_id"`
	ID            types.Int64  `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Code          types.String `tfsdk:"code"`
	AlternateCode types.String `tfsdk:"alternate_code"`
}

func (d *regionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_region"
}

func (d *regionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge provider region by ID. Regions are scoped to a server provider (e.g. DigitalOcean, AWS), not to an organization.",
		Attributes: map[string]schema.Attribute{
			"provider_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server provider that offers the region (Forge API `provider` path segment; `provider` is reserved in HCL).", Required: true},
			"id":             schema.Int64Attribute{MarkdownDescription: "Numeric ID of the provider region.", Required: true},
			"name":           schema.StringAttribute{MarkdownDescription: "Human-readable region name (e.g. `Amsterdam 2`).", Computed: true},
			"code":           schema.StringAttribute{MarkdownDescription: "Region code used when provisioning (e.g. `ams2`).", Computed: true},
			"alternate_code": schema.StringAttribute{MarkdownDescription: "Alternate region code, if the provider exposes one.", Computed: true},
		},
	}
}

func (d *regionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *regionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data regionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Provider regions are provider-scoped (the resource's links.self), not
	// org-scoped: /api/providers/{provider}/regions/{region}.
	path := fmt.Sprintf("/api/providers/%d/regions/%d", data.ProviderID.ValueInt64(), data.ID.ValueInt64())
	var a regionAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge provider region", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Code = types.StringValue(a.Code)
	data.AlternateCode = types.StringPointerValue(a.AlternateCode)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
