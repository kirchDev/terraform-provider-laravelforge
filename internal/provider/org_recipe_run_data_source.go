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
	_ datasource.DataSource              = (*orgRecipeRunDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgRecipeRunDataSource)(nil)
)

// NewOrgRecipeRunDataSource returns a new laravelforge_org_recipe_run data source.
func NewOrgRecipeRunDataSource() datasource.DataSource {
	return &orgRecipeRunDataSource{}
}

type orgRecipeRunDataSource struct {
	client *client.Client
}

// orgRecipeRunAttributes mirrors the JSON:API "attributes" of a recipeLogs resource.
type orgRecipeRunAttributes struct {
	ServerID   int64   `json:"server_id"`
	ExecutedBy *int64  `json:"executed_by"`
	RecipeID   int64   `json:"recipe_id"`
	Status     string  `json:"status"`
	Output     *string `json:"output"`
	StartedAt  *string `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
}

type orgRecipeRunDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	Recipe       types.Int64  `tfsdk:"recipe"`
	ID           types.Int64  `tfsdk:"id"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ExecutedBy   types.Int64  `tfsdk:"executed_by"`
	RecipeID     types.Int64  `tfsdk:"recipe_id"`
	Status       types.String `tfsdk:"status"`
	Output       types.String `tfsdk:"output"`
	StartedAt    types.String `tfsdk:"started_at"`
	FinishedAt   types.String `tfsdk:"finished_at"`
}

func (d *orgRecipeRunDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_recipe_run"
}

func (d *orgRecipeRunDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge recipe run (log) by ID for a recipe in an organization.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"recipe":       schema.Int64Attribute{MarkdownDescription: "Numeric ID of the recipe.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the recipe run (log).", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the recipe ran on.", Computed: true},
			"executed_by":  schema.Int64Attribute{MarkdownDescription: "ID of the user who executed the run.", Computed: true},
			"recipe_id":    schema.Int64Attribute{MarkdownDescription: "ID of the recipe that was run.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Run status (`waiting`, `running`, `finished`, `failed`).", Computed: true},
			"output":       schema.StringAttribute{MarkdownDescription: "Output captured from the run.", Computed: true},
			"started_at":   schema.StringAttribute{MarkdownDescription: "Timestamp the run started.", Computed: true},
			"finished_at":  schema.StringAttribute{MarkdownDescription: "Timestamp the run finished.", Computed: true},
		},
	}
}

func (d *orgRecipeRunDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgRecipeRunDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgRecipeRunDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/recipes/%d/runs/%d", data.Organization.ValueString(), data.Recipe.ValueInt64(), data.ID.ValueInt64())
	var a orgRecipeRunAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge recipe run", err.Error())
		return
	}

	data.ServerID = types.Int64Value(a.ServerID)
	data.ExecutedBy = types.Int64PointerValue(a.ExecutedBy)
	data.RecipeID = types.Int64Value(a.RecipeID)
	data.Status = types.StringValue(a.Status)
	data.Output = types.StringPointerValue(a.Output)
	data.StartedAt = types.StringPointerValue(a.StartedAt)
	data.FinishedAt = types.StringPointerValue(a.FinishedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
