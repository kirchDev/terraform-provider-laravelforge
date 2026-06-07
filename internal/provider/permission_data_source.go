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
	_ datasource.DataSource              = (*permissionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*permissionDataSource)(nil)
)

// NewPermissionDataSource returns a new laravelforge_permission data source.
func NewPermissionDataSource() datasource.DataSource {
	return &permissionDataSource{}
}

type permissionDataSource struct {
	client *client.Client
}

// permissionAttributes mirrors the JSON:API "attributes" of a permission
// resource (Roles tag, global catalog of assignable permissions).
type permissionAttributes struct {
	Name string `json:"name"`
}

type permissionDataSourceModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *permissionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_permission"
}

func (d *permissionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single assignable permission from the global Laravel Forge permissions catalog by ID.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.Int64Attribute{MarkdownDescription: "Numeric ID of the permission.", Required: true},
			"name": schema.StringAttribute{MarkdownDescription: "Permission name.", Computed: true},
		},
	}
}

func (d *permissionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *permissionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data permissionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Permissions are a global catalog, not org-scoped.
	path := fmt.Sprintf("/api/permissions/%d", data.ID.ValueInt64())
	var a permissionAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge permission", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
