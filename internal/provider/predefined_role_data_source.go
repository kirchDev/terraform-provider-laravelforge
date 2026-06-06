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
	_ datasource.DataSource              = (*predefinedRoleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*predefinedRoleDataSource)(nil)
)

// NewPredefinedRoleDataSource returns a new laravelforge_predefined_role data source.
func NewPredefinedRoleDataSource() datasource.DataSource {
	return &predefinedRoleDataSource{}
}

type predefinedRoleDataSource struct {
	client *client.Client
}

// predefinedRoleAttributes mirrors the JSON:API "attributes" of a predefined role.
type predefinedRoleAttributes struct {
	Name      string  `json:"name"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type predefinedRoleDataSourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *predefinedRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_predefined_role"
}

func (d *predefinedRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single built-in predefined role from the global Forge catalog by ID.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{MarkdownDescription: "Numeric ID of the predefined role.", Required: true},
			"name":       schema.StringAttribute{MarkdownDescription: "Predefined role name.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (d *predefinedRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *predefinedRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data predefinedRoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Global catalog: predefined roles are not org-scoped.
	path := fmt.Sprintf("/api/predefined-roles/%d", data.ID.ValueInt64())
	var a predefinedRoleAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge predefined role", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
