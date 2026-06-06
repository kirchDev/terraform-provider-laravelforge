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

// --- Singleton-resource pattern. The deployment script has no own id and no
// create/destroy endpoint: it's a single object hanging off a site, edited via
// GET (show) + PUT (update). Identity is the parent ids only; create == update
// (PUT), and remove is a no-op (Forge keeps the script — there's no destroy). ---

var (
	_ resource.Resource                = (*siteDeployScriptResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteDeployScriptResource)(nil)
	_ resource.ResourceWithImportState = (*siteDeployScriptResource)(nil)
)

// NewSiteDeployScriptResource returns a new laravelforge_site_deploy_script resource.
func NewSiteDeployScriptResource() resource.Resource {
	return &siteDeployScriptResource{}
}

type siteDeployScriptResource struct {
	client *client.Client
}

// siteDeployScriptAttributes mirrors the JSON:API "attributes" of a deployment
// script (read shape).
type siteDeployScriptAttributes struct {
	Content    *string `json:"content"`
	AutoSource bool    `json:"auto_source"`
}

type siteDeployScriptResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Content      types.String `tfsdk:"content"`
	AutoSource   types.Bool   `tfsdk:"auto_source"`
}

func (r *siteDeployScriptResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_deploy_script"
}

func (r *siteDeployScriptResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the deployment script of a Laravel Forge site. " +
			"The script is a singleton on the site (no separate ID): it is read via " +
			"`GET` and replaced via `PUT`, and removing the resource leaves the script " +
			"in place (Forge has no destroy endpoint for it).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the site belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"site_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the site whose deployment script is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "The content of the deployment script.",
				Required:            true,
			},
			"auto_source": schema.BoolAttribute{
				MarkdownDescription: "Make `.env` variables available to the deployment script.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *siteDeployScriptResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// scriptPath is the singleton path for the site's deployment script.
func (r *siteDeployScriptResource) scriptPath(m *siteDeployScriptResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/deployments/script",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

// write PUTs the planned script and refreshes computed fields from the response.
func (r *siteDeployScriptResource) write(ctx context.Context, m *siteDeployScriptResourceModel) error {
	body := map[string]any{"content": m.Content.ValueString()}
	if !m.AutoSource.IsNull() && !m.AutoSource.IsUnknown() {
		body["auto_source"] = m.AutoSource.ValueBool()
	}

	var a siteDeployScriptAttributes
	if _, err := r.client.Write(ctx, "PUT", r.scriptPath(m), body, &a); err != nil {
		return err
	}
	r.apply(m, &a)
	return nil
}

// apply copies response attributes onto the model. content stays as planned when
// the API echoes null (the PUT body is authoritative).
func (r *siteDeployScriptResource) apply(m *siteDeployScriptResourceModel, a *siteDeployScriptAttributes) {
	if a.Content != nil {
		m.Content = types.StringValue(*a.Content)
	}
	m.AutoSource = types.BoolValue(a.AutoSource)
}

func (r *siteDeployScriptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteDeployScriptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge deployment script", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDeployScriptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteDeployScriptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var a siteDeployScriptAttributes
	if _, err := r.client.Get(ctx, r.scriptPath(&state), &a); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge deployment script", err.Error())
		return
	}
	r.apply(&state, &a)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteDeployScriptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteDeployScriptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge deployment script", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the deployment script is a singleton with no destroy
// endpoint, so removing the resource just drops it from state and leaves the
// current script untouched on Forge.
func (r *siteDeployScriptResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteDeployScriptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id\".")
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
}
