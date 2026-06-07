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
	_ datasource.DataSource              = (*recipeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*recipeDataSource)(nil)
)

// NewRecipeDataSource returns a new laravelforge_recipe data source.
func NewRecipeDataSource() datasource.DataSource {
	return &recipeDataSource{}
}

type recipeDataSource struct {
	client *client.Client
}

type recipeDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	User         types.String `tfsdk:"user"`
	Script       types.String `tfsdk:"script"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *recipeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recipe"
}

func (d *recipeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge recipe by ID within an organization.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the recipe.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Recipe name.", Computed: true},
			"user":         schema.StringAttribute{MarkdownDescription: "User the recipe runs as (`root` or `forge`).", Computed: true},
			"script":       schema.StringAttribute{MarkdownDescription: "Shell script executed by the recipe.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *recipeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *recipeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data recipeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/recipes/%d", data.Organization.ValueString(), data.ID.ValueInt64())
	var a recipeAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge recipe", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.User = types.StringValue(a.User)
	data.Script = types.StringValue(a.Script)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
