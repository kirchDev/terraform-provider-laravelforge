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
	_ resource.Resource                = (*orgTeamInviteResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgTeamInviteResource)(nil)
	_ resource.ResourceWithImportState = (*orgTeamInviteResource)(nil)
)

// NewOrgTeamInviteResource returns a new laravelforge_org_team_invite resource.
func NewOrgTeamInviteResource() resource.Resource {
	return &orgTeamInviteResource{}
}

type orgTeamInviteResource struct {
	client *client.Client
}

// orgTeamInviteAttributes mirrors the JSON:API "attributes" of a team
// invitation (read shape). role_id lives under relationships, not attributes,
// so it is not read back here — it is a create-only input.
type orgTeamInviteAttributes struct {
	Email     string  `json:"email"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type orgTeamInviteResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	TeamID       types.Int64  `tfsdk:"team_id"`
	ID           types.Int64  `tfsdk:"id"`
	RoleID       types.Int64  `tfsdk:"role_id"`
	Email        types.String `tfsdk:"email"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *orgTeamInviteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_invite"
}

func (r *orgTeamInviteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a pending invitation to a Laravel Forge team. The API has no update endpoint, so any change recreates the invitation.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the team to invite the member to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the invitation.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"role_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the role to grant the invited member. Create-only; changing it recreates the invitation.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address of the invited member. Create-only; changing it recreates the invitation.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *orgTeamInviteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *orgTeamInviteResource) basePath(m *orgTeamInviteResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/teams/%d/invites", m.Organization.ValueString(), m.TeamID.ValueInt64())
}

func (r *orgTeamInviteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgTeamInviteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"role_id": plan.RoleID.ValueInt64(),
		"email":   plan.Email.ValueString(),
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge team invite", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected team invite ID", fmt.Sprintf("Forge returned a non-numeric invitation ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read team invite after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgTeamInviteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgTeamInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge team invite", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: every writable attribute is RequiresReplace and the API
// has no update endpoint. It exists only to satisfy the resource interface.
func (r *orgTeamInviteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Forge team invitations cannot be updated in place; changes recreate the invitation.")
}

func (r *orgTeamInviteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgTeamInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge team invite", err.Error())
	}
}

// ImportState accepts "organization/team_id/invitation_id". role_id is not
// readable from the API, so an imported resource will show role_id drift until
// the configured value matches; it is documented as create-only.
func (r *orgTeamInviteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/team_id/invitation_id\".")
		return
	}
	teamID, err1 := strconv.ParseInt(parts[1], 10, 64)
	invitationID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "team_id and invitation_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), teamID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), invitationID)...)
}

// readInto GETs the invitation identified by m and fills the computed fields.
// role_id is not present in the response attributes (it lives under
// relationships), so it is left untouched on the model.
func (r *orgTeamInviteResource) readInto(ctx context.Context, m *orgTeamInviteResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a orgTeamInviteAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Email = types.StringValue(a.Email)
	m.CreatedAt = types.StringPointerValue(a.CreatedAt)
	m.UpdatedAt = types.StringPointerValue(a.UpdatedAt)
	return nil
}
