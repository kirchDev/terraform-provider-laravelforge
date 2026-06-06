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
	_ resource.Resource                = (*orgServerCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgServerCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*orgServerCredentialResource)(nil)
)

// NewOrgServerCredentialResource returns a new laravelforge_org_server_credential
// resource.
//
// This manages the *share linkage* between a team and an org-scoped server
// credential — not the credential object itself (read-only via the
// laravelforge_org_server_credential data source / laravelforge_credential).
// Create POSTs a ShareCredentialRequest (`{credential_id}`) to the team's
// server-credentials collection; Delete DELETEs the share under the same team
// path. There is no update endpoint (update_method: none) — any change to the
// identity recreates the share.
func NewOrgServerCredentialResource() resource.Resource {
	return &orgServerCredentialResource{}
}

type orgServerCredentialResource struct {
	client *client.Client
}

// orgServerCredentialAttributes mirrors the JSON:API "attributes" of a
// ServerCredentialResource (the read/share response shape).
type orgServerCredentialAttributes struct {
	Name      string  `json:"name"`
	Provider  string  `json:"provider"`
	InUse     bool    `json:"in_use"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

// orgServerCredentialResourceModel is the state shape. Identity is the parent
// ids plus the credential id; all are RequiresReplace (no update endpoint).
type orgServerCredentialResourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	TeamID        types.Int64  `tfsdk:"team_id"`
	CredentialID  types.Int64  `tfsdk:"credential_id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	InUse         types.Bool   `tfsdk:"in_use"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *orgServerCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_server_credential"
}

func (r *orgServerCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shares an organization server credential with a team in Laravel Forge. This manages the share linkage only (created on POST, removed on DELETE); the credential object itself is read-only. There is no update — any change recreates the share.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the team the credential is shared with.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"credential_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server credential to share with the team.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name":           schema.StringAttribute{MarkdownDescription: "Credential name.", Computed: true},
			"cloud_provider": schema.StringAttribute{MarkdownDescription: "Provider the credential authenticates against, e.g. `Hetzner Cloud` (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
			"in_use":         schema.BoolAttribute{MarkdownDescription: "Whether the credential is currently in use by a server.", Computed: true},
			"created_at":     schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":     schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *orgServerCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// teamPath is the team-scoped collection used to create/delete the share.
func (r *orgServerCredentialResource) teamPath(m *orgServerCredentialResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/teams/%d/server-credentials", m.Organization.ValueString(), m.TeamID.ValueInt64())
}

func (r *orgServerCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgServerCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Share = POST ShareCredentialRequest ({credential_id}) to the team's
	// server-credentials collection.
	body := map[string]any{"credential_id": plan.CredentialID.ValueInt64()}
	if _, err := r.client.Write(ctx, "POST", r.teamPath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to share Forge server credential with team", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read server credential after share", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgServerCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgServerCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge server credential", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can't happen: every input attribute is RequiresReplace and the share
// has no update endpoint, so changes recreate the resource. Defined to satisfy
// the interface.
func (r *orgServerCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgServerCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server credential", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgServerCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgServerCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Unshare = DELETE under the team path with the credential id.
	itemPath := fmt.Sprintf("%s/%d", r.teamPath(&state), state.CredentialID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to unshare Forge server credential", err.Error())
	}
}

// ImportState accepts "organization/team_id/credential_id".
func (r *orgServerCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/team_id/credential_id\".")
		return
	}
	teamID, err1 := strconv.ParseInt(parts[1], 10, 64)
	credentialID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "team_id and credential_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), teamID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("credential_id"), credentialID)...)
}

// readInto verifies the share still exists and fills the computed credential
// fields. The share itself has no single-read endpoint, so this lists the
// team's shared credentials (the share linkage) and looks up this credential;
// if it's absent the share was removed out of band — surfaced as NotFound so
// Read drops it from state. The org-scoped single-read provides the credential
// attributes.
func (r *orgServerCredentialResource) readInto(ctx context.Context, m *orgServerCredentialResourceModel) error {
	want := strconv.FormatInt(m.CredentialID.ValueInt64(), 10)
	shares, err := r.client.List(ctx, r.teamPath(m))
	if err != nil {
		return err
	}
	found := false
	for _, s := range shares {
		if s.ID == want {
			found = true
			break
		}
	}
	if !found {
		return &client.APIError{StatusCode: 404, Method: "GET", Path: r.teamPath(m), Body: "credential not shared with team"}
	}

	readPath := fmt.Sprintf("/api/orgs/%s/server-credentials/%d", m.Organization.ValueString(), m.CredentialID.ValueInt64())
	var a orgServerCredentialAttributes
	if _, err := r.client.Get(ctx, readPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.CloudProvider = types.StringValue(a.Provider)
	m.InUse = types.BoolValue(a.InUse)
	m.CreatedAt = types.StringPointerValue(a.CreatedAt)
	m.UpdatedAt = types.StringPointerValue(a.UpdatedAt)
	return nil
}
