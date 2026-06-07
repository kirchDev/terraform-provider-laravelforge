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
	_ datasource.DataSource              = (*forgeRecipeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*forgeRecipeDataSource)(nil)
)

// NewForgeRecipeDataSource returns a new laravelforge_forge_recipe data source.
func NewForgeRecipeDataSource() datasource.DataSource {
	return &forgeRecipeDataSource{}
}

type forgeRecipeDataSource struct {
	client *client.Client
}

// forgeRecipeAttributes mirrors the JSON:API "attributes" of a forgeRecipes
// resource (the global catalog of first-party Forge-provided recipes).
type forgeRecipeAttributes struct {
	Name      string  `json:"name"`
	User      string  `json:"user"`
	Info      string  `json:"info"`
	Script    string  `json:"script"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type forgeRecipeDataSourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	User      types.String `tfsdk:"user"`
	Info      types.String `tfsdk:"info"`
	Script    types.String `tfsdk:"script"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *forgeRecipeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forge_recipe"
}

func (d *forgeRecipeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single first-party Forge-provided recipe from the global catalog by ID. Read-only; distinct from an organization's own recipes.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{MarkdownDescription: "Numeric ID of the Forge recipe.", Required: true},
			"name":       schema.StringAttribute{MarkdownDescription: "Recipe name.", Computed: true},
			"user":       schema.StringAttribute{MarkdownDescription: "User the recipe runs as.", Computed: true},
			"info":       schema.StringAttribute{MarkdownDescription: "Recipe description / info.", Computed: true},
			"script":     schema.StringAttribute{MarkdownDescription: "Shell script the recipe executes.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *forgeRecipeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *forgeRecipeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data forgeRecipeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/forge-recipes/%d", data.ID.ValueInt64())
	var a forgeRecipeAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge recipe", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.User = types.StringValue(a.User)
	data.Info = types.StringValue(a.Info)
	data.Script = types.StringValue(a.Script)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
