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

// --- Singleton-resource pattern. The nginx config block is a singleton per
// domain record: GET show + PUT update only, no own id, no destroy. Identity
// is the parent ids (organization/server_id/site_id/domain_record_id). ---

var (
	_ resource.Resource                = (*siteDomainNginxResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteDomainNginxResource)(nil)
	_ resource.ResourceWithImportState = (*siteDomainNginxResource)(nil)
)

// NewSiteDomainNginxResource returns a new laravelforge_site_domain_nginx resource.
func NewSiteDomainNginxResource() resource.Resource {
	return &siteDomainNginxResource{}
}

type siteDomainNginxResource struct {
	client *client.Client
}

// siteDomainNginxAttributes mirrors the JSON:API "attributes" of a domain
// nginx config (read shape).
type siteDomainNginxAttributes struct {
	Content *string `json:"content"`
}

type siteDomainNginxResourceModel struct {
	Organization   types.String `tfsdk:"organization"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	SiteID         types.Int64  `tfsdk:"site_id"`
	DomainRecordID types.Int64  `tfsdk:"domain_record_id"`
	Config         types.String `tfsdk:"config"`
	Content        types.String `tfsdk:"content"`
}

func (r *siteDomainNginxResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_domain_nginx"
}

func (r *siteDomainNginxResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the per-domain nginx configuration block on a Laravel Forge site. Singleton per domain record (PUT update only, no separate create/destroy).",
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
				MarkdownDescription: "ID of the site the domain record belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"domain_record_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the domain record whose nginx config this manages.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "Nginx configuration block to apply for the domain.",
				Required:            true,
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "Current nginx configuration content as reported by Forge.",
				Computed:            true,
			},
		},
	}
}

func (r *siteDomainNginxResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteDomainNginxResource) singletonPath(m *siteDomainNginxResourceModel) string {
	return fmt.Sprintf(
		"/api/orgs/%s/servers/%d/sites/%d/domains/%d/nginx",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64(), m.DomainRecordID.ValueInt64(),
	)
}

// write PUTs the planned config to the singleton path, then reads it back.
func (r *siteDomainNginxResource) write(ctx context.Context, m *siteDomainNginxResourceModel) error {
	body := map[string]any{"config": m.Config.ValueString()}
	if _, err := r.client.Write(ctx, "PUT", r.singletonPath(m), body, nil); err != nil {
		return err
	}
	return r.readInto(ctx, m)
}

func (r *siteDomainNginxResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteDomainNginxResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge domain nginx config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteDomainNginxResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteDomainNginxResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge domain nginx config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteDomainNginxResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteDomainNginxResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge domain nginx config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the domain nginx config is a singleton with no destroy
// endpoint (PUT update only). Removing it from state leaves Forge's config
// in place.
func (r *siteDomainNginxResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/site_id/domain_record_id".
func (r *siteDomainNginxResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/site_id/domain_record_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	siteID, err2 := strconv.ParseInt(parts[2], 10, 64)
	domainRecordID, err3 := strconv.ParseInt(parts[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id, site_id and domain_record_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_record_id"), domainRecordID)...)
}

// readInto GETs the singleton nginx config identified by m and fills the
// computed content field.
func (r *siteDomainNginxResource) readInto(ctx context.Context, m *siteDomainNginxResourceModel) error {
	var a siteDomainNginxAttributes
	if _, err := r.client.Get(ctx, r.singletonPath(m), &a); err != nil {
		return err
	}
	m.Content = types.StringPointerValue(a.Content)
	return nil
}
