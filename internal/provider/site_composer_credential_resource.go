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

// --- CRUD-resource pattern: a Composer auth credential on a Forge site, keyed
// by repository host. The id is the {repository} string (also the item-path
// key). Create=POST collection, Update=PUT item, Delete=DELETE item.
// repository is the identity (RequiresReplace); username/password are mutable.
// password is sensitive. ---

var (
	_ resource.Resource                = (*siteComposerCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteComposerCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*siteComposerCredentialResource)(nil)
)

// NewSiteComposerCredentialResource returns a new laravelforge_site_composer_credential resource.
func NewSiteComposerCredentialResource() resource.Resource {
	return &siteComposerCredentialResource{}
}

type siteComposerCredentialResource struct {
	client *client.Client
}

// siteComposerCredentialAttributes mirrors the JSON:API "attributes" of a
// Composer credential (read shape).
type siteComposerCredentialAttributes struct {
	Repository string `json:"repository"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type siteComposerCredentialResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Repository   types.String `tfsdk:"repository"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
}

func (r *siteComposerCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_composer_credential"
}

func (r *siteComposerCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Composer authentication credential for a repository host on a Laravel Forge site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the server the site belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the site the credential belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "Repository host the credential authenticates against (the credential's identity).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username for the Composer credential.",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password (or token) for the Composer credential.",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *siteComposerCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// basePath is the site-scoped collection path; Create (POST) lives here.
func (r *siteComposerCredentialResource) basePath(m *siteComposerCredentialResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/composer/credentials",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

// itemPath is the per-repository route used for read/update/delete.
func (r *siteComposerCredentialResource) itemPath(m *siteComposerCredentialResourceModel) string {
	return fmt.Sprintf("%s/%s", r.basePath(m), m.Repository.ValueString())
}

func (r *siteComposerCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteComposerCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"repository": plan.Repository.ValueString(),
		"username":   plan.Username.ValueString(),
		"password":   plan.Password.ValueString(),
	}
	if _, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge composer credential", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read composer credential after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteComposerCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteComposerCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge composer credential", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteComposerCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteComposerCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"repository": plan.Repository.ValueString(),
		"username":   plan.Username.ValueString(),
		"password":   plan.Password.ValueString(),
	}
	if _, err := r.client.Write(ctx, "PUT", r.itemPath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge composer credential", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read composer credential after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteComposerCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteComposerCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.itemPath(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge composer credential", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id/repository".
func (r *siteComposerCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 4)
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id/repository\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	siteID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and site_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repository"), parts[3])...)
}

// readInto GETs the credential identified by m and fills the read fields.
func (r *siteComposerCredentialResource) readInto(ctx context.Context, m *siteComposerCredentialResourceModel) error {
	var a siteComposerCredentialAttributes
	if _, err := r.client.Get(ctx, r.itemPath(m), &a); err != nil {
		return err
	}
	m.Repository = types.StringValue(a.Repository)
	m.Username = types.StringValue(a.Username)
	m.Password = types.StringValue(a.Password)
	return nil
}
