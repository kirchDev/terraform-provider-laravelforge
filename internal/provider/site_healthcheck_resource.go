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

// --- Singleton-resource pattern. The healthcheck endpoint config is a single
// object hanging off a site: GET show + PUT update only (no create/list/delete
// endpoints). So identity is the parent ids (organization/server_id/site_id),
// there is no own "id" attribute, Create == Update == PUT, and Delete is a
// no-op (PUT a null endpoint to clear it). ---

var (
	_ resource.Resource                = (*siteHealthcheckResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteHealthcheckResource)(nil)
	_ resource.ResourceWithImportState = (*siteHealthcheckResource)(nil)
)

// NewSiteHealthcheckResource returns a new laravelforge_site_healthcheck resource.
func NewSiteHealthcheckResource() resource.Resource {
	return &siteHealthcheckResource{}
}

type siteHealthcheckResource struct {
	client *client.Client
}

// siteHealthcheckAttributes mirrors the JSON:API "attributes" of the
// healthcheck endpoint resource (read shape).
type siteHealthcheckAttributes struct {
	HealthcheckEndpoint *string `json:"healthcheck_endpoint"`
}

type siteHealthcheckResourceModel struct {
	Organization        types.String `tfsdk:"organization"`
	ServerID            types.Int64  `tfsdk:"server_id"`
	SiteID              types.Int64  `tfsdk:"site_id"`
	HealthcheckEndpoint types.String `tfsdk:"healthcheck_endpoint"`
}

func (r *siteHealthcheckResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_healthcheck"
}

func (r *siteHealthcheckResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the healthcheck endpoint configuration for a Laravel Forge site (singleton).",
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
				MarkdownDescription: "ID of the site whose healthcheck endpoint is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"healthcheck_endpoint": schema.StringAttribute{
				MarkdownDescription: "The endpoint / URL used to perform healthchecks (e.g. `https://my-app.com/up`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteHealthcheckResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteHealthcheckResource) healthcheckPath(m *siteHealthcheckResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/healthcheck", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

// write PUTs the (flat) healthcheck endpoint body. The PUT returns 204 No
// Content, so the caller reads back to populate computed state.
func (r *siteHealthcheckResource) write(ctx context.Context, m *siteHealthcheckResourceModel) error {
	body := map[string]any{}
	if m.HealthcheckEndpoint.IsNull() || m.HealthcheckEndpoint.IsUnknown() {
		body["healthcheck_endpoint"] = nil
	} else {
		body["healthcheck_endpoint"] = m.HealthcheckEndpoint.ValueString()
	}
	_, err := r.client.Write(ctx, "PUT", r.healthcheckPath(m), body, nil)
	return err
}

func (r *siteHealthcheckResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteHealthcheckResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge site healthcheck endpoint", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read healthcheck endpoint after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteHealthcheckResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteHealthcheckResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site healthcheck endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteHealthcheckResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteHealthcheckResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge site healthcheck endpoint", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read healthcheck endpoint after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete clears the healthcheck endpoint (no DELETE endpoint exists): PUT a
// null endpoint so the site no longer has one configured.
func (r *siteHealthcheckResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteHealthcheckResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.HealthcheckEndpoint = types.StringNull()
	if err := r.write(ctx, &state); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to clear Forge site healthcheck endpoint", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteHealthcheckResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the site's healthcheck endpoint and fills the model.
func (r *siteHealthcheckResource) readInto(ctx context.Context, m *siteHealthcheckResourceModel) error {
	var a siteHealthcheckAttributes
	if _, err := r.client.Get(ctx, r.healthcheckPath(m), &a); err != nil {
		return err
	}
	m.HealthcheckEndpoint = types.StringPointerValue(a.HealthcheckEndpoint)
	return nil
}
