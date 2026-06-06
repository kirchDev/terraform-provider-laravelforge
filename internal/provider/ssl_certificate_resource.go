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

// SSL certificates live UNDER a domain in the new (org-scoped) Forge API:
//
//	GET/POST   /api/orgs/{org}/servers/{server}/sites/{site}/domains/{domain}/certificates
//	GET/DELETE /api/orgs/{org}/servers/{server}/sites/{site}/domains/{domain}/certificates/{id}
//
// Verified live (2026-06-06): the certificate item route reports
// "Supported methods: GET, HEAD, DELETE" and the collection reports
// "GET, HEAD, POST" — so a certificate is create + read + delete only. There
// is no PUT/PATCH and no .../activate sub-route, hence no Update: every input
// is RequiresReplace.

var (
	_ resource.Resource                = (*sslCertificateResource)(nil)
	_ resource.ResourceWithConfigure   = (*sslCertificateResource)(nil)
	_ resource.ResourceWithImportState = (*sslCertificateResource)(nil)
)

// NewSSLCertificateResource returns a new laravelforge_ssl_certificate resource.
func NewSSLCertificateResource() resource.Resource {
	return &sslCertificateResource{}
}

type sslCertificateResource struct {
	client *client.Client
}

// sslCertificateAttributes mirrors the JSON:API "attributes" of a certificate.
// Only scalar fields are mapped (per the spec for this pass).
type sslCertificateAttributes struct {
	Domain        string  `json:"domain"`
	Type          string  `json:"type"`
	RequestStatus *string `json:"request_status"`
	Status        *string `json:"status"`
	Active        bool    `json:"active"`
	Existing      bool    `json:"existing"`
	CreatedAt     *string `json:"created_at"`
}

type sslCertificateResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	DomainID     types.Int64  `tfsdk:"domain_id"`
	ID           types.Int64  `tfsdk:"id"`

	// Create-only inputs (FLAT write body). RequiresReplace.
	Type             types.String `tfsdk:"type"`
	Domain           types.String `tfsdk:"domain"`
	Key              types.String `tfsdk:"key"`
	Certificate      types.String `tfsdk:"certificate"`
	CertificateID    types.Int64  `tfsdk:"certificate_id"`
	Country          types.String `tfsdk:"country"`
	State            types.String `tfsdk:"state"`
	City             types.String `tfsdk:"city"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Department       types.String `tfsdk:"department"`

	// Computed read-back scalars.
	RequestStatus types.String `tfsdk:"request_status"`
	Status        types.String `tfsdk:"status"`
	Active        types.Bool   `tfsdk:"active"`
	Existing      types.Bool   `tfsdk:"existing"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func (r *sslCertificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssl_certificate"
}

func (r *sslCertificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SSL/TLS certificate on a Laravel Forge site domain. Certificates are create + read + delete only (the Forge API exposes no update for them), so every input forces replacement.",
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
				MarkdownDescription: "ID of the site the domain belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"domain_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the domain the certificate is installed on (certificates are scoped per-domain in the new Forge SSL system).",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the certificate.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "Certificate type to create: `new` (CSR / Let's Encrypt), `existing` (upload key + certificate), or `clone` (copy from another certificate). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Primary domain the certificate covers. Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Private key (PEM) for an `existing` certificate. Create-only, write-only.",
				Optional:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"certificate": schema.StringAttribute{
				MarkdownDescription: "Certificate body (PEM, including intermediates) for an `existing` certificate. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"certificate_id": schema.Int64Attribute{
				MarkdownDescription: "Source certificate ID to copy when `type = clone`. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"country": schema.StringAttribute{
				MarkdownDescription: "Two-letter country code for a `new` (CSR) certificate. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "State / province for a `new` (CSR) certificate. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"city": schema.StringAttribute{
				MarkdownDescription: "City / locality for a `new` (CSR) certificate. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"organization_name": schema.StringAttribute{
				MarkdownDescription: "Organization name for a `new` (CSR) certificate. Sent to the Forge API as `organization` (renamed here to avoid colliding with the `organization` slug). Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"department": schema.StringAttribute{
				MarkdownDescription: "Organizational unit / department for a `new` (CSR) certificate. Create-only.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},

			"request_status": schema.StringAttribute{MarkdownDescription: "Certificate request status.", Computed: true},
			"status":         schema.StringAttribute{MarkdownDescription: "Certificate status.", Computed: true},
			"active":         schema.BoolAttribute{MarkdownDescription: "Whether this certificate is the active one for the domain.", Computed: true},
			"existing":       schema.BoolAttribute{MarkdownDescription: "Whether the certificate was uploaded (vs. issued by Forge).", Computed: true},
			"created_at":     schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *sslCertificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// basePath is the per-domain certificate collection (where create + list live).
func (r *sslCertificateResource) basePath(m *sslCertificateResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/domains/%d/certificates",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64(), m.DomainID.ValueInt64())
}

func (r *sslCertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sslCertificateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"type": plan.Type.ValueString()}
	if !plan.Domain.IsNull() && !plan.Domain.IsUnknown() {
		body["domain"] = plan.Domain.ValueString()
	}
	if !plan.Key.IsNull() && !plan.Key.IsUnknown() {
		body["key"] = plan.Key.ValueString()
	}
	if !plan.Certificate.IsNull() && !plan.Certificate.IsUnknown() {
		body["certificate"] = plan.Certificate.ValueString()
	}
	if !plan.CertificateID.IsNull() && !plan.CertificateID.IsUnknown() {
		body["certificate_id"] = plan.CertificateID.ValueInt64()
	}
	if !plan.Country.IsNull() && !plan.Country.IsUnknown() {
		body["country"] = plan.Country.ValueString()
	}
	if !plan.State.IsNull() && !plan.State.IsUnknown() {
		body["state"] = plan.State.ValueString()
	}
	if !plan.City.IsNull() && !plan.City.IsUnknown() {
		body["city"] = plan.City.ValueString()
	}
	if !plan.OrganizationName.IsNull() && !plan.OrganizationName.IsUnknown() {
		body["organization"] = plan.OrganizationName.ValueString()
	}
	if !plan.Department.IsNull() && !plan.Department.IsUnknown() {
		body["department"] = plan.Department.ValueString()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge SSL certificate", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected certificate ID", fmt.Sprintf("Forge returned a non-numeric certificate ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read SSL certificate after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sslCertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sslCertificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge SSL certificate", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never be called: every attribute is RequiresReplace and the API
// exposes no PUT/PATCH for certificates. Present only to satisfy the interface.
func (r *sslCertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"SSL certificate update not supported",
		"Forge SSL certificates are immutable; changes must replace the resource. This is a provider bug if reached.",
	)
}

func (r *sslCertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sslCertificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge SSL certificate", err.Error())
	}
}

// ImportState accepts "organization/server_id/site_id/domain_id/id".
func (r *sslCertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 5 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id/domain_id/id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	siteID, err2 := strconv.ParseInt(parts[2], 10, 64)
	domainID, err3 := strconv.ParseInt(parts[3], 10, 64)
	certID, err4 := strconv.ParseInt(parts[4], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id, site_id, domain_id and id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_id"), domainID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), certID)...)
}

// readInto GETs the certificate identified by m and fills computed fields. The
// single-certificate read path mirrors the collection (per-domain, same scope
// as create/delete): .../domains/{domain}/certificates/{id}.
func (r *sslCertificateResource) readInto(ctx context.Context, m *sslCertificateResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a sslCertificateAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Domain = types.StringValue(a.Domain)
	// `type` is a Required, create-only input (the user's new/existing/clone
	// choice); the read-back value can differ in vocabulary, so it is NOT
	// overwritten here to avoid drift on a RequiresReplace attribute.
	m.RequestStatus = types.StringPointerValue(a.RequestStatus)
	m.Status = types.StringPointerValue(a.Status)
	m.Active = types.BoolValue(a.Active)
	m.Existing = types.BoolValue(a.Existing)
	m.CreatedAt = types.StringPointerValue(a.CreatedAt)
	return nil
}
