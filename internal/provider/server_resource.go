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

// --- CRUD-resource for a Laravel Forge server (org-scoped). Mirrors the
// site_resource.go shape. Verified against the live API (2026-06-06):
//   - create:  POST   /api/orgs/{org}/servers           (flat body)
//   - read:    GET    /api/orgs/{org}/servers/{id}       (links.self; org-level)
//   - update:  PUT    /api/orgs/{org}/servers/{id}       (flat body)
//   - delete:  DELETE /api/orgs/{org}/servers/{id}
//
// Only scalar attributes are mapped. Provider-specific create blocks
// (hetzner/aws/ocean2/vultr/akamai/laravel/custom) and the tags array are
// objects/arrays and are skipped in this pass.

var (
	_ resource.Resource                = (*serverResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverResource)(nil)
	_ resource.ResourceWithImportState = (*serverResource)(nil)
)

// NewServerResource returns a new laravelforge_server resource.
func NewServerResource() resource.Resource {
	return &serverResource{}
}

type serverResource struct {
	client *client.Client
}

// serverResourceModel mirrors the scalar attributes of a server. Reuses the
// serverAttributes read struct defined in server_data_source.go, extended here
// with the extra scalars this resource manages (ubuntu_version, timezone,
// database_type, php_version) via serverResourceAttributes.
type serverResourceModel struct {
	Organization     types.String `tfsdk:"organization"`
	ID               types.Int64  `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	Type             types.String `tfsdk:"type"`
	CloudProvider    types.String `tfsdk:"cloud_provider"`
	UbuntuVersion    types.String `tfsdk:"ubuntu_version"`
	Region           types.String `tfsdk:"region"`
	Size             types.String `tfsdk:"size"`
	CredentialID     types.Int64  `tfsdk:"credential_id"`
	PHPVersion       types.String `tfsdk:"php_version"`
	DatabaseType     types.String `tfsdk:"database_type"`
	IPAddress        types.String `tfsdk:"ip_address"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	Timezone         types.String `tfsdk:"timezone"`
	IsReady          types.Bool   `tfsdk:"is_ready"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

// serverResourceAttributes mirrors the JSON:API "attributes" of a server,
// including the scalars this resource manages beyond the data-source set.
type serverResourceAttributes struct {
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Type             string  `json:"type"`
	Provider         string  `json:"provider"`
	UbuntuVersion    *string `json:"ubuntu_version"`
	Region           string  `json:"region"`
	Size             string  `json:"size"`
	CredentialID     *int64  `json:"credential_id"`
	PHPVersion       *string `json:"php_version"`
	DatabaseType     *string `json:"database_type"`
	IPAddress        *string `json:"ip_address"`
	PrivateIPAddress *string `json:"private_ip_address"`
	Timezone         *string `json:"timezone"`
	IsReady          bool    `json:"is_ready"`
	CreatedAt        string  `json:"created_at"`
}

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Laravel Forge server within an organization. Provider-specific provisioning blocks (hetzner, aws, …) and tags are not modelled in this pass.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the server.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Server name (max 30 characters). Updatable.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Server type (e.g. `app`, `web`, `database`). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_provider": schema.StringAttribute{
				MarkdownDescription: "Underlying server provider (Forge API `provider`; renamed because `provider` is reserved in HCL). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ubuntu_version": schema.StringAttribute{
				MarkdownDescription: "Ubuntu version (e.g. `22.04`, `24.04`). Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"credential_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the provider credential to provision with. Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace(), int64planmodifier.UseStateForUnknown()},
			},
			"php_version": schema.StringAttribute{
				MarkdownDescription: "PHP version key (e.g. `php82`). Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"database_type": schema.StringAttribute{
				MarkdownDescription: "Database engine key (e.g. `postgres15`). Create-only.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "Public IP address. Updatable.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"private_ip_address": schema.StringAttribute{
				MarkdownDescription: "Private IP address. Updatable.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "Server timezone. Updatable.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug":       schema.StringAttribute{MarkdownDescription: "Server slug.", Computed: true},
			"region":     schema.StringAttribute{MarkdownDescription: "Region the server runs in.", Computed: true},
			"size":       schema.StringAttribute{MarkdownDescription: "Server size / plan.", Computed: true},
			"is_ready":   schema.BoolAttribute{MarkdownDescription: "Whether the server has finished provisioning.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverResource) basePath(m *serverResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers", m.Organization.ValueString())
}

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"provider": plan.CloudProvider.ValueString(),
		"type":     plan.Type.ValueString(),
	}
	if !plan.UbuntuVersion.IsNull() && !plan.UbuntuVersion.IsUnknown() {
		body["ubuntu_version"] = plan.UbuntuVersion.ValueString()
	}
	if !plan.CredentialID.IsNull() && !plan.CredentialID.IsUnknown() {
		body["credential_id"] = plan.CredentialID.ValueInt64()
	}
	if !plan.PHPVersion.IsNull() && !plan.PHPVersion.IsUnknown() {
		body["php_version"] = plan.PHPVersion.ValueString()
	}
	if !plan.DatabaseType.IsNull() && !plan.DatabaseType.IsUnknown() {
		body["database_type"] = plan.DatabaseType.ValueString()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge server", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected server ID", fmt.Sprintf("Forge returned a non-numeric server ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read server after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge server", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only name, ip_address, private_ip_address and timezone are updatable; all
	// other writable fields are RequiresReplace.
	body := map[string]any{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.IPAddress.IsNull() && !plan.IPAddress.IsUnknown() {
		body["ip_address"] = plan.IPAddress.ValueString()
	}
	if !plan.PrivateIPAddress.IsNull() && !plan.PrivateIPAddress.IsUnknown() {
		body["private_ip_address"] = plan.PrivateIPAddress.ValueString()
	}
	if !plan.Timezone.IsNull() && !plan.Timezone.IsUnknown() {
		body["timezone"] = plan.Timezone.ValueString()
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge server", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read server after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge server", err.Error())
	}
}

// ImportState accepts "organization/id".
func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/id\".")
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// readInto GETs the server identified by m and fills the computed/optional
// fields. The single-server read is org-level (per the resource's links.self,
// confirmed against the live API).
func (r *serverResource) readInto(ctx context.Context, m *serverResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a serverResourceAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Slug = types.StringValue(a.Slug)
	m.Type = types.StringValue(a.Type)
	m.CloudProvider = types.StringValue(a.Provider)
	m.UbuntuVersion = types.StringPointerValue(a.UbuntuVersion)
	m.Region = types.StringValue(a.Region)
	m.Size = types.StringValue(a.Size)
	m.CredentialID = types.Int64PointerValue(a.CredentialID)
	m.PHPVersion = types.StringPointerValue(a.PHPVersion)
	m.DatabaseType = types.StringPointerValue(a.DatabaseType)
	m.IPAddress = types.StringPointerValue(a.IPAddress)
	m.PrivateIPAddress = types.StringPointerValue(a.PrivateIPAddress)
	m.Timezone = types.StringPointerValue(a.Timezone)
	m.IsReady = types.BoolValue(a.IsReady)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	return nil
}
