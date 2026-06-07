package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Link resource: shares an existing recipe with a team. No PUT exists, so
// any change recreates (all identity inputs are RequiresReplace). There is no
// single-share GET; Read lists the team's recipes and checks membership. ---

var (
	_ resource.Resource                = (*orgTeamRecipeShareResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgTeamRecipeShareResource)(nil)
	_ resource.ResourceWithImportState = (*orgTeamRecipeShareResource)(nil)
)

// NewOrgTeamRecipeShareResource returns a new laravelforge_org_team_recipe_share resource.
func NewOrgTeamRecipeShareResource() resource.Resource {
	return &orgTeamRecipeShareResource{}
}

type orgTeamRecipeShareResource struct {
	client *client.Client
}

type orgTeamRecipeShareResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	TeamID       types.Int64  `tfsdk:"team_id"`
	RecipeID     types.Int64  `tfsdk:"recipe_id"`
	Name         types.String `tfsdk:"name"`
	Script       types.String `tfsdk:"script"`
	User         types.String `tfsdk:"user"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *orgTeamRecipeShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_recipe_share"
}

func (r *orgTeamRecipeShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shares an existing recipe with a team in a Laravel Forge organization. " +
			"This is a link between a team and a recipe; the recipe itself is managed elsewhere. " +
			"There is no update endpoint, so changing any attribute recreates the share.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the team the recipe is shared with.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"recipe_id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the recipe to share with the team.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name":       schema.StringAttribute{MarkdownDescription: "Name of the shared recipe.", Computed: true},
			"script":     schema.StringAttribute{MarkdownDescription: "Shell script executed by the recipe.", Computed: true},
			"user":       schema.StringAttribute{MarkdownDescription: "User the recipe runs as.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Recipe creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Recipe last-update timestamp.", Computed: true},
		},
	}
}

func (r *orgTeamRecipeShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *orgTeamRecipeShareResource) basePath(m *orgTeamRecipeShareResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/teams/%d/recipes", m.Organization.ValueString(), m.TeamID.ValueInt64())
}

func (r *orgTeamRecipeShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgTeamRecipeShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"recipe_id": plan.RecipeID.ValueInt64()}
	if _, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to share Forge recipe with team", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read recipe share after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamRecipeShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgTeamRecipeShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge recipe share", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable: every input is RequiresReplace (no PUT exists), so the
// framework always recreates instead of updating.
func (r *orgTeamRecipeShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgTeamRecipeShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read recipe share after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamRecipeShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgTeamRecipeShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.RecipeID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to unshare Forge recipe", err.Error())
	}
}

// ImportState accepts "organization/team_id/recipe_id".
func (r *orgTeamRecipeShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/team_id/recipe_id\".")
		return
	}
	teamID, err1 := strconv.ParseInt(parts[1], 10, 64)
	recipeID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "team_id and recipe_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), teamID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("recipe_id"), recipeID)...)
}

// readInto lists the team's shared recipes and fills the computed fields from the
// one matching RecipeID. There is no single-share GET, so membership is checked
// against the index; a missing entry surfaces as a not-found error.
func (r *orgTeamRecipeShareResource) readInto(ctx context.Context, m *orgTeamRecipeShareResourceModel) error {
	a, err := findSharedRecipe(ctx, r.client, m.Organization.ValueString(), m.TeamID.ValueInt64(), m.RecipeID.ValueInt64())
	if err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Script = types.StringValue(a.Script)
	m.User = types.StringValue(a.User)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}

// findSharedRecipe scans the team's recipe index for recipeID. It returns the
// recipe's attributes, or a not-found *client.APIError if the recipe is not
// shared with the team (so callers can drop the resource from state).
func findSharedRecipe(ctx context.Context, c *client.Client, org string, teamID, recipeID int64) (recipeAttributes, error) {
	listPath := fmt.Sprintf("/api/orgs/%s/teams/%d/recipes", org, teamID)
	items, err := c.List(ctx, listPath)
	if err != nil {
		return recipeAttributes{}, err
	}
	want := strconv.FormatInt(recipeID, 10)
	for _, it := range items {
		if it.ID != want {
			continue
		}
		var a recipeAttributes
		if err := json.Unmarshal(it.Attributes, &a); err != nil {
			return recipeAttributes{}, fmt.Errorf("decoding recipe %s attributes: %w", it.ID, err)
		}
		return a, nil
	}
	return recipeAttributes{}, &client.APIError{
		StatusCode: http.StatusNotFound,
		Method:     "GET",
		Path:       listPath,
		Body:       fmt.Sprintf("recipe %d is not shared with team %d", recipeID, teamID),
	}
}
