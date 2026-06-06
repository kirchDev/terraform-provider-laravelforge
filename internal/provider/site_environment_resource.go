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

// --- Singleton-resource pattern: the site's .env file (one per site). No own
// id; identity is the parent ids (organization/server_id/site_id). Read=GET the
// environment path, Create==Update=PUT it, Delete is a no-op (the .env always
// exists for a site; we just stop managing it). Content is sensitive. ---

var (
	_ resource.Resource                = (*siteEnvironmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteEnvironmentResource)(nil)
	_ resource.ResourceWithImportState = (*siteEnvironmentResource)(nil)
)

// NewSiteEnvironmentResource returns a new laravelforge_site_environment resource.
func NewSiteEnvironmentResource() resource.Resource {
	return &siteEnvironmentResource{}
}

type siteEnvironmentResource struct {
	client *client.Client
}

// siteEnvironmentAttributes mirrors the JSON:API "attributes" of the
// environment resource (read shape): the rendered .env file contents.
type siteEnvironmentAttributes struct {
	Content string `json:"content"`
}

type siteEnvironmentResourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	SiteID        types.Int64  `tfsdk:"site_id"`
	Environment   types.String `tfsdk:"environment"`
	Cache         types.Bool   `tfsdk:"cache"`
	Queues        types.Bool   `tfsdk:"queues"`
	EncryptionKey types.String `tfsdk:"encryption_key"`
	Content       types.String `tfsdk:"content"`
}

func (r *siteEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_environment"
}

func (r *siteEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the `.env` file contents of a Laravel Forge site (singleton per site).",
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
				MarkdownDescription: "ID of the site whose environment is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"environment": schema.StringAttribute{
				MarkdownDescription: "The `.env` file contents to write to the site.",
				Required:            true,
				Sensitive:           true,
			},
			"cache": schema.BoolAttribute{
				MarkdownDescription: "Whether to cache the configuration after updating the environment.",
				Optional:            true,
			},
			"queues": schema.BoolAttribute{
				MarkdownDescription: "Whether to restart the site's queues after updating the environment.",
				Optional:            true,
			},
			"encryption_key": schema.StringAttribute{
				MarkdownDescription: "Optional encryption key used to encrypt the environment file.",
				Optional:            true,
				Sensitive:           true,
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "The current `.env` file contents as returned by Forge.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteEnvironmentResource) path(m *siteEnvironmentResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/environment", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.SiteID.ValueInt64())
}

// write PUTs the planned environment (Create and Update share this).
func (r *siteEnvironmentResource) write(ctx context.Context, m *siteEnvironmentResourceModel) error {
	body := map[string]any{"environment": m.Environment.ValueString()}
	if !m.Cache.IsNull() && !m.Cache.IsUnknown() {
		body["cache"] = m.Cache.ValueBool()
	}
	if !m.Queues.IsNull() && !m.Queues.IsUnknown() {
		body["queues"] = m.Queues.ValueBool()
	}
	if !m.EncryptionKey.IsNull() && !m.EncryptionKey.IsUnknown() {
		body["encryption_key"] = m.EncryptionKey.ValueString()
	}
	_, err := r.client.Write(ctx, "PUT", r.path(m), body, nil)
	return err
}

func (r *siteEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge site environment", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read site environment after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge site environment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge site environment", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read site environment after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: a site's .env always exists, so there is nothing to
// destroy. Removing the resource just stops Terraform managing the contents.
func (r *siteEnvironmentResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/site_id".
func (r *siteEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the environment singleton identified by m and fills content.
func (r *siteEnvironmentResource) readInto(ctx context.Context, m *siteEnvironmentResourceModel) error {
	var a siteEnvironmentAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Content = types.StringValue(a.Content)
	return nil
}
