package provider

import (
	"context"
	"fmt"
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

var (
	_ resource.Resource                = (*orgTeamMemberResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgTeamMemberResource)(nil)
	_ resource.ResourceWithImportState = (*orgTeamMemberResource)(nil)
)

// NewOrgTeamMemberResource returns a new laravelforge_org_team_member resource.
func NewOrgTeamMemberResource() resource.Resource {
	return &orgTeamMemberResource{}
}

type orgTeamMemberResource struct {
	client *client.Client
}

// orgTeamMemberAttributes mirrors the JSON:API "attributes" of a
// MembershipResource (read shape). The member's role lives under
// "relationships.role" (an object) and is intentionally not mapped here; the
// writable role_id is carried on the model instead.
type orgTeamMemberAttributes struct {
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type orgTeamMemberResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	TeamID       types.Int64  `tfsdk:"team_id"`
	ID           types.Int64  `tfsdk:"id"`
	RoleID       types.Int64  `tfsdk:"role_id"`
	Name         types.String `tfsdk:"name"`
	Email        types.String `tfsdk:"email"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *orgTeamMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_member"
}

func (r *orgTeamMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a user's membership and role within a Laravel Forge team. " +
			"There is no create endpoint — a member joins a team through the invite flow " +
			"(`laravelforge_org_team_invite`); this resource adopts an existing membership and " +
			"manages its role (PUT) and removal (DELETE). It is commonly imported.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the team the membership belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the user whose membership is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"role_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the role assigned to the member within the team.",
				Required:            true,
			},
			"name":       schema.StringAttribute{MarkdownDescription: "Member's name.", Computed: true},
			"email":      schema.StringAttribute{MarkdownDescription: "Member's email address.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Membership creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Membership update timestamp.", Computed: true},
		},
	}
}

func (r *orgTeamMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *orgTeamMemberResource) itemPath(m *orgTeamMemberResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/teams/%d/members/%d", m.Organization.ValueString(), m.TeamID.ValueInt64(), m.ID.ValueInt64())
}

// Create adopts an existing membership: there is no POST, so the member must
// already exist (via the invite flow). It PUTs the desired role onto that
// membership and reads it back.
func (r *orgTeamMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgTeamMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"role_id": plan.RoleID.ValueInt64()}
	if _, err := r.client.Write(ctx, "PUT", r.itemPath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to assign Forge team member role", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read team member after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgTeamMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge team member", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *orgTeamMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgTeamMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"role_id": plan.RoleID.ValueInt64()}
	if _, err := r.client.Write(ctx, "PUT", r.itemPath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge team member", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read team member after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgTeamMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.itemPath(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge team member", err.Error())
	}
}

// ImportState accepts "organization/team_id/user_id".
func (r *orgTeamMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/team_id/user_id\".")
		return
	}
	teamID, err1 := strconv.ParseInt(parts[1], 10, 64)
	userID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "team_id and user_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), teamID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), userID)...)
}

// readInto GETs the membership identified by m and fills the computed fields.
// role_id is not present in the JSON:API attributes (it lives under
// relationships.role), so it is left as configured/imported.
func (r *orgTeamMemberResource) readInto(ctx context.Context, m *orgTeamMemberResourceModel) error {
	var a orgTeamMemberAttributes
	if _, err := r.client.Get(ctx, r.itemPath(m), &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Email = types.StringValue(a.Email)
	m.CreatedAt = types.StringPointerValue(a.CreatedAt)
	m.UpdatedAt = types.StringPointerValue(a.UpdatedAt)
	return nil
}
