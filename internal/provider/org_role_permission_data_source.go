package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ datasource.DataSource              = (*orgRolePermissionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgRolePermissionDataSource)(nil)
)

// NewOrgRolePermissionDataSource returns a new laravelforge_org_role_permission
// data source.
func NewOrgRolePermissionDataSource() datasource.DataSource {
	return &orgRolePermissionDataSource{}
}

type orgRolePermissionDataSource struct {
	client *client.Client
}

// orgRolePermissionAttributes mirrors the JSON:API "attributes" of a single
// PermissionResource.
type orgRolePermissionAttributes struct {
	Name string `json:"name"`
}

// orgRolePermissionModel is one entry of the computed permissions list.
type orgRolePermissionModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type orgRolePermissionDataSourceModel struct {
	Organization types.String             `tfsdk:"organization"`
	Role         types.Int64              `tfsdk:"role"`
	Permissions  []orgRolePermissionModel `tfsdk:"permissions"`
}

func (d *orgRolePermissionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_role_permission"
}

func (d *orgRolePermissionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the permissions attached to a given Laravel Forge organization role.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"role":         schema.Int64Attribute{MarkdownDescription: "Numeric ID of the role whose permissions to list.", Required: true},
			"permissions": schema.ListNestedAttribute{
				MarkdownDescription: "Permissions attached to the role.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{MarkdownDescription: "ID of the permission.", Computed: true},
						"name": schema.StringAttribute{MarkdownDescription: "Name of the permission.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *orgRolePermissionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgRolePermissionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgRolePermissionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/roles/%d/permissions", data.Organization.ValueString(), data.Role.ValueInt64())
	resources, err := d.client.List(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge role permissions", err.Error())
		return
	}

	data.Permissions = make([]orgRolePermissionModel, 0, len(resources))
	for _, r := range resources {
		var a orgRolePermissionAttributes
		if len(r.Attributes) > 0 {
			if err := json.Unmarshal(r.Attributes, &a); err != nil {
				resp.Diagnostics.AddError("Unable to decode Forge role permission", err.Error())
				return
			}
		}
		data.Permissions = append(data.Permissions, orgRolePermissionModel{
			ID:   types.StringValue(r.ID),
			Name: types.StringValue(a.Name),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
