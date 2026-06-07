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
	_ datasource.DataSource              = (*orgStorageProviderDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgStorageProviderDataSource)(nil)
)

// NewOrgStorageProviderDataSource returns a new laravelforge_org_storage_provider data source.
func NewOrgStorageProviderDataSource() datasource.DataSource {
	return &orgStorageProviderDataSource{}
}

type orgStorageProviderDataSource struct {
	client *client.Client
}

type orgStorageProviderDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ID            types.Int64  `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	ProviderName  types.String `tfsdk:"provider_name"`
	Region        types.String `tfsdk:"region"`
	Bucket        types.String `tfsdk:"bucket"`
	Directory     types.String `tfsdk:"directory"`
	Endpoint      types.String `tfsdk:"endpoint"`
	AssumeRole    types.Bool   `tfsdk:"assume_role"`
	InUse         types.Bool   `tfsdk:"in_use"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (d *orgStorageProviderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_storage_provider"
}

func (d *orgStorageProviderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge organization storage provider by ID.",
		Attributes: map[string]schema.Attribute{
			"organization":   schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"id":             schema.Int64Attribute{MarkdownDescription: "Numeric ID of the storage provider.", Required: true},
			"name":           schema.StringAttribute{MarkdownDescription: "Display name of the storage provider.", Computed: true},
			"cloud_provider": schema.StringAttribute{MarkdownDescription: "Storage provider type (`s3`, `spaces`, `hetzner`, `ovh`, `scaleway`, `custom`).", Computed: true},
			"provider_name":  schema.StringAttribute{MarkdownDescription: "Human-readable provider name (e.g. `Amazon S3`).", Computed: true},
			"region":         schema.StringAttribute{MarkdownDescription: "Storage region.", Computed: true},
			"bucket":         schema.StringAttribute{MarkdownDescription: "Bucket name.", Computed: true},
			"directory":      schema.StringAttribute{MarkdownDescription: "Directory (path prefix) within the bucket.", Computed: true},
			"endpoint":       schema.StringAttribute{MarkdownDescription: "Custom S3-compatible endpoint URL.", Computed: true},
			"assume_role":    schema.BoolAttribute{MarkdownDescription: "Whether an EC2 assumed IAM role is used instead of access/secret keys.", Computed: true},
			"in_use":         schema.BoolAttribute{MarkdownDescription: "Whether the storage provider is currently used by a backup configuration.", Computed: true},
			"created_at":     schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":     schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *orgStorageProviderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgStorageProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgStorageProviderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/storage-providers/%d", data.Organization.ValueString(), data.ID.ValueInt64())
	var a orgStorageProviderAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge storage provider", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.CloudProvider = types.StringValue(a.Provider)
	data.ProviderName = types.StringValue(a.ProviderName)
	data.Region = types.StringPointerValue(a.Region)
	data.Bucket = types.StringPointerValue(a.Bucket)
	data.Directory = types.StringPointerValue(a.Directory)
	data.Endpoint = types.StringPointerValue(a.Endpoint)
	data.AssumeRole = types.BoolPointerValue(a.AssumeRole)
	data.InUse = types.BoolValue(a.InUse)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
