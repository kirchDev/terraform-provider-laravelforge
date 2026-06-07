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
	_ resource.Resource                = (*siteNpmCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteNpmCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*siteNpmCredentialResource)(nil)
)

// NewSiteNpmCredentialResource returns a new laravelforge_site_npm_credential
// resource.
func NewSiteNpmCredentialResource() resource.Resource {
	return &siteNpmCredentialResource{}
}

type siteNpmCredentialResource struct {
	client *client.Client
}

// siteNpmCredentialAttributes mirrors the JSON:API "attributes" of an NPM
// credential. "scopes" is an array attribute and is skipped (scalars only).
type siteNpmCredentialAttributes struct {
	Registry string `json:"registry"`
	Token    string `json:"token"`
}

type siteNpmCredentialResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Registry     types.String `tfsdk:"registry"`
	Token        types.String `tfsdk:"token"`
}

func (r *siteNpmCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_npm_credential"
}

func (r *siteNpmCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages NPM registry authentication credentials for a Laravel Forge site, keyed by registry.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server hosting the site.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the site the credential belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"registry": schema.StringAttribute{
				MarkdownDescription: "The NPM registry URL the credential authenticates against. Acts as the credential's identifier.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "The authentication token for the registry.",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *siteNpmCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteNpmCredentialResource) basePath(m *siteNpmCredentialResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/npm/credentials", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteNpmCredentialResource) itemPath(m *siteNpmCredentialResourceModel) string {
	return fmt.Sprintf("%s/%s", r.basePath(m), m.Registry.ValueString())
}

func (r *siteNpmCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteNpmCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"registry": plan.Registry.ValueString(),
		"token":    plan.Token.ValueString(),
	}
	if _, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge NPM credential", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read NPM credential after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteNpmCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteNpmCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge NPM credential", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteNpmCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteNpmCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"registry": plan.Registry.ValueString(),
		"token":    plan.Token.ValueString(),
	}
	if _, err := r.client.Write(ctx, "PUT", r.itemPath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge NPM credential", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read NPM credential after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteNpmCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteNpmCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.itemPath(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge NPM credential", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id/registry".
func (r *siteNpmCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 4)
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id/registry\".")
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("registry"), parts[3])...)
}

// readInto GETs the credential identified by m and fills the computed fields.
// "token" is not overwritten from the API response: it is the configured
// sensitive value and Forge does not return the secret on read.
func (r *siteNpmCredentialResource) readInto(ctx context.Context, m *siteNpmCredentialResourceModel) error {
	var a siteNpmCredentialAttributes
	if _, err := r.client.Get(ctx, r.itemPath(m), &a); err != nil {
		return err
	}
	if a.Registry != "" {
		m.Registry = types.StringValue(a.Registry)
	}
	return nil
}
