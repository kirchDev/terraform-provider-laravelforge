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
	_ datasource.DataSource              = (*providerSizeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*providerSizeDataSource)(nil)
)

// NewProviderSizeDataSource returns a new laravelforge_provider_size data source.
func NewProviderSizeDataSource() datasource.DataSource {
	return &providerSizeDataSource{}
}

type providerSizeDataSource struct {
	client *client.Client
}

// providerSizeAttributes mirrors the JSON:API "attributes" of a provider size
// resource (read at /api/providers/{provider}/sizes/{providerSize}).
type providerSizeAttributes struct {
	Name         string `json:"name"`
	Code         string `json:"code"`
	Series       string `json:"series"`
	Category     string `json:"category"`
	CPUs         int64  `json:"cpus"`
	DiskType     string `json:"disk_type"`
	Architecture string `json:"architecture"`
	RAM          int64  `json:"ram"`
	Disk         int64  `json:"disk"`
}

type providerSizeDataSourceModel struct {
	ProviderID   types.Int64  `tfsdk:"provider_id"`
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Code         types.String `tfsdk:"code"`
	Series       types.String `tfsdk:"series"`
	Category     types.String `tfsdk:"category"`
	CPUs         types.Int64  `tfsdk:"cpus"`
	DiskType     types.String `tfsdk:"disk_type"`
	Architecture types.String `tfsdk:"architecture"`
	RAM          types.Int64  `tfsdk:"ram"`
	Disk         types.Int64  `tfsdk:"disk"`
}

func (d *providerSizeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_size"
}

func (d *providerSizeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge provider size (server plan) by ID. Sizes are scoped to a server provider (e.g. DigitalOcean, AWS), not to an organization.",
		Attributes: map[string]schema.Attribute{
			"provider_id":  schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server provider that offers the size (Forge API `provider` path segment; `provider` is reserved in HCL).", Required: true},
			"id":           schema.StringAttribute{MarkdownDescription: "Forge ID of the provider size.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Human-readable name of the size.", Computed: true},
			"code":         schema.StringAttribute{MarkdownDescription: "Code identifier from the provider.", Computed: true},
			"series":       schema.StringAttribute{MarkdownDescription: "Series type of the size.", Computed: true},
			"category":     schema.StringAttribute{MarkdownDescription: "Category name of the size.", Computed: true},
			"cpus":         schema.Int64Attribute{MarkdownDescription: "Number of CPUs.", Computed: true},
			"disk_type":    schema.StringAttribute{MarkdownDescription: "Type of disk.", Computed: true},
			"architecture": schema.StringAttribute{MarkdownDescription: "CPU architecture.", Computed: true},
			"ram":          schema.Int64Attribute{MarkdownDescription: "Amount of RAM in MB.", Computed: true},
			"disk":         schema.Int64Attribute{MarkdownDescription: "Amount of disk space in MB.", Computed: true},
		},
	}
}

func (d *providerSizeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *providerSizeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data providerSizeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Provider sizes are provider-scoped (the resource's links.self), not
	// org-scoped: /api/providers/{provider}/sizes/{providerSize}.
	path := fmt.Sprintf("/api/providers/%d/sizes/%s", data.ProviderID.ValueInt64(), data.ID.ValueString())
	var a providerSizeAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge provider size", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Code = types.StringValue(a.Code)
	data.Series = types.StringValue(a.Series)
	data.Category = types.StringValue(a.Category)
	data.CPUs = types.Int64Value(a.CPUs)
	data.DiskType = types.StringValue(a.DiskType)
	data.Architecture = types.StringValue(a.Architecture)
	data.RAM = types.Int64Value(a.RAM)
	data.Disk = types.Int64Value(a.Disk)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
