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

var (
	_ resource.Resource                = (*siteDomainResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteDomainResource)(nil)
	_ resource.ResourceWithImportState = (*siteDomainResource)(nil)
)

// NewSiteDomainResource returns a new laravelforge_site_domain resource.
func NewSiteDomainResource() resource.Resource {
	return &siteDomainResource{}
}

type siteDomainResource struct {
	client *client.Client
}

// siteDomainAttributes mirrors the JSON:API "attributes" of a domain record.
type siteDomainAttributes struct {
	Name                    string `json:"name"`
	Type                    string `json:"type"`
	Status                  string `json:"status"`
	WwwRedirectType         string `json:"www_redirect_type"`
	AllowWildcardSubdomains bool   `json:"allow_wildcard_subdomains"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
}

type siteDomainResourceModel struct {
	Organization            types.String `tfsdk:"organization"`
	ServerID                types.Int64  `tfsdk:"server_id"`
	SiteID                  types.Int64  `tfsdk:"site_id"`
	ID                      types.Int64  `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Type                    types.String `tfsdk:"type"`
	Status                  types.String `tfsdk:"status"`
	WwwRedirectType         types.String `tfsdk:"www_redirect_type"`
	AllowWildcardSubdomains types.Bool   `tfsdk:"allow_wildcard_subdomains"`
	CreatedAt               types.String `tfsdk:"created_at"`
	UpdatedAt               types.String `tfsdk:"updated_at"`
}

func (r *siteDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_domain"
}

func (r *siteDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a domain record on a Laravel Forge site.",
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
				MarkdownDescription: "ID of the site the domain belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the domain record.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the domain (e.g. `laravel.com`). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"www_redirect_type": schema.StringAttribute{
				MarkdownDescription: "Type of `www` redirection (`from-www`, `to-www`, or `none`).",
				Required:            true,
			},
			"allow_wildcard_subdomains": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain allows wildcard subdomains.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"type":       schema.StringAttribute{MarkdownDescription: "The type of domain (`primary` or `alias`).", Computed: true},
			"status":     schema.StringAttribute{MarkdownDescription: "The status of the domain.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last-updated timestamp.", Computed: true},
		},
	}
}

func (r *siteDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteDomainResource) basePath(m *siteDomainResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/domains", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

func (r *siteDomainResource) itemPath(m *siteDomainResourceModel) string {
	return fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
}

func (r *siteDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":              plan.Name.ValueString(),
		"www_redirect_type": plan.WwwRedirectType.ValueString(),
	}
	if !plan.AllowWildcardSubdomains.IsNull() && !plan.AllowWildcardSubdomains.IsUnknown() {
		body["allow_wildcard_subdomains"] = plan.AllowWildcardSubdomains.ValueBool()
	} else {
		body["allow_wildcard_subdomains"] = false
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge site domain", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected domain ID", fmt.Sprintf("Forge returned a non-numeric domain ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read site domain after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site domain", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"www_redirect_type": plan.WwwRedirectType.ValueString(),
	}
	if !plan.AllowWildcardSubdomains.IsNull() && !plan.AllowWildcardSubdomains.IsUnknown() {
		body["allow_wildcard_subdomains"] = plan.AllowWildcardSubdomains.ValueBool()
	}

	if _, err := r.client.Write(ctx, "PATCH", r.itemPath(&plan), body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge site domain", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read site domain after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.itemPath(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge site domain", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id/domain_id".
func (r *siteDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id/domain_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	siteID, err2 := strconv.ParseInt(parts[2], 10, 64)
	domainID, err3 := strconv.ParseInt(parts[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id, site_id and domain_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domainID)...)
}

// readInto GETs the domain record identified by m and fills the computed fields.
func (r *siteDomainResource) readInto(ctx context.Context, m *siteDomainResourceModel) error {
	var a siteDomainAttributes
	if _, err := r.client.Get(ctx, r.itemPath(m), &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Type = types.StringValue(a.Type)
	m.Status = types.StringValue(a.Status)
	m.WwwRedirectType = types.StringValue(a.WwwRedirectType)
	m.AllowWildcardSubdomains = types.BoolValue(a.AllowWildcardSubdomains)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}
