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
	_ datasource.DataSource              = (*orgTeamRecipeShareDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgTeamRecipeShareDataSource)(nil)
)

// NewOrgTeamRecipeShareDataSource returns a new laravelforge_org_team_recipe_share data source.
func NewOrgTeamRecipeShareDataSource() datasource.DataSource {
	return &orgTeamRecipeShareDataSource{}
}

type orgTeamRecipeShareDataSource struct {
	client *client.Client
}

type orgTeamRecipeShareDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	TeamID       types.Int64  `tfsdk:"team_id"`
	RecipeID     types.Int64  `tfsdk:"recipe_id"`
	Name         types.String `tfsdk:"name"`
	Script       types.String `tfsdk:"script"`
	User         types.String `tfsdk:"user"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *orgTeamRecipeShareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_recipe_share"
}

func (d *orgTeamRecipeShareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a recipe that is shared with a team in a Laravel Forge organization.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"team_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the team the recipe is shared with.", Required: true},
			"recipe_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the shared recipe.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Name of the shared recipe.", Computed: true},
			"script":       schema.StringAttribute{MarkdownDescription: "Shell script executed by the recipe.", Computed: true},
			"user":         schema.StringAttribute{MarkdownDescription: "User the recipe runs as.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Recipe creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Recipe last-update timestamp.", Computed: true},
		},
	}
}

func (d *orgTeamRecipeShareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgTeamRecipeShareDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgTeamRecipeShareDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	a, err := findSharedRecipe(ctx, d.client, data.Organization.ValueString(), data.TeamID.ValueInt64(), data.RecipeID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge recipe share", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Script = types.StringValue(a.Script)
	data.User = types.StringValue(a.User)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
