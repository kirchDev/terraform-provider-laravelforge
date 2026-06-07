package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- CRUD-resource pattern exemplar. New resources follow this shape. ---

var (
	_ resource.Resource                = (*siteResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteResource)(nil)
	_ resource.ResourceWithImportState = (*siteResource)(nil)
)

// NewSiteResource returns a new laravelforge_site resource.
func NewSiteResource() resource.Resource {
	return &siteResource{}
}

type siteResource struct {
	client *client.Client
}

// siteAttributes mirrors the JSON:API "attributes" of a site (read shape).
type siteAttributes struct {
	Name                    string  `json:"name"`
	PHPVersion              *string `json:"php_version"`
	RootDirectory           *string `json:"root_directory"`
	WebDirectory            *string `json:"web_directory"`
	Status                  string  `json:"status"`
	URL                     *string `json:"url"`
	ZeroDowntimeDeployments bool    `json:"zero_downtime_deployments"`
	CreatedAt               string  `json:"created_at"`
}

type siteResourceModel struct {
	Organization            types.String `tfsdk:"organization"`
	ServerID                types.Int64  `tfsdk:"server_id"`
	ID                      types.Int64  `tfsdk:"id"`
	Type                    types.String `tfsdk:"type"`
	Name                    types.String `tfsdk:"name"`
	PHPVersion              types.String `tfsdk:"php_version"`
	RootDirectory           types.String `tfsdk:"root_directory"`
	WebDirectory            types.String `tfsdk:"web_directory"`
	Status                  types.String `tfsdk:"status"`
	URL                     types.String `tfsdk:"url"`
	ZeroDowntimeDeployments types.Bool   `tfsdk:"zero_downtime_deployments"`
	CreatedAt               types.String `tfsdk:"created_at"`
}

func (r *siteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (r *siteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a site on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to create the site on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Site type (e.g. `php`). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the site.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Site domain name.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"php_version": schema.StringAttribute{
				MarkdownDescription: "PHP version key (e.g. `php82`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"root_directory": schema.StringAttribute{
				MarkdownDescription: "Project root directory.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"web_directory": schema.StringAttribute{
				MarkdownDescription: "Web directory served by nginx.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zero_downtime_deployments": schema.BoolAttribute{
				MarkdownDescription: "Whether zero-downtime deployments are enabled.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"status":     schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"url":        schema.StringAttribute{MarkdownDescription: "Site URL.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *siteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteResource) basePath(m *siteResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *siteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"type": plan.Type.ValueString()}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.PHPVersion.IsNull() && !plan.PHPVersion.IsUnknown() {
		body["php_version"] = plan.PHPVersion.ValueString()
	}
	if !plan.RootDirectory.IsNull() && !plan.RootDirectory.IsUnknown() {
		body["root_directory"] = plan.RootDirectory.ValueString()
	}
	if !plan.WebDirectory.IsNull() && !plan.WebDirectory.IsUnknown() {
		body["web_directory"] = plan.WebDirectory.ValueString()
	}
	if !plan.ZeroDowntimeDeployments.IsNull() && !plan.ZeroDowntimeDeployments.IsUnknown() {
		body["zero_downtime_deployments"] = plan.ZeroDowntimeDeployments.ValueBool()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge site", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected site ID", fmt.Sprintf("Forge returned a non-numeric site ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read site after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.PHPVersion.IsNull() && !plan.PHPVersion.IsUnknown() {
		body["php_version"] = plan.PHPVersion.ValueString()
	}
	if !plan.RootDirectory.IsNull() && !plan.RootDirectory.IsUnknown() {
		body["root_directory"] = plan.RootDirectory.ValueString()
	}
	if !plan.WebDirectory.IsNull() && !plan.WebDirectory.IsUnknown() {
		body["web_directory"] = plan.WebDirectory.ValueString()
	}
	if !plan.ZeroDowntimeDeployments.IsNull() && !plan.ZeroDowntimeDeployments.IsUnknown() {
		body["zero_downtime_deployments"] = plan.ZeroDowntimeDeployments.ValueBool()
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge site", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read site after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge site", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), siteID)...)
}

// readInto GETs the site identified by m and fills the computed/optional fields.
// Single-site reads are org-level (per the resource's links.self), unlike
// create/update/delete which are server-scoped.
func (r *siteResource) readInto(ctx context.Context, m *siteResourceModel) error {
	itemPath := fmt.Sprintf("/api/orgs/%s/sites/%d", m.Organization.ValueString(), m.ID.ValueInt64())
	var a siteAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.PHPVersion = types.StringPointerValue(a.PHPVersion)
	m.RootDirectory = types.StringPointerValue(a.RootDirectory)
	m.WebDirectory = types.StringPointerValue(a.WebDirectory)
	m.Status = types.StringValue(a.Status)
	m.URL = types.StringPointerValue(a.URL)
	m.ZeroDowntimeDeployments = types.BoolValue(a.ZeroDowntimeDeployments)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	return nil
}
