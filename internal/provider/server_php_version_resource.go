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
	_ resource.Resource                = (*serverPHPVersionResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPVersionResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPVersionResource)(nil)
)

// NewServerPhpVersionResource returns a new laravelforge_server_php_version resource.
func NewServerPhpVersionResource() resource.Resource {
	return &serverPHPVersionResource{}
}

type serverPHPVersionResource struct {
	client *client.Client
}

// serverPHPVersionAttributes mirrors the JSON:API "attributes" of an installed
// PHP version (read shape).
type serverPHPVersionAttributes struct {
	Version    string `json:"version"`
	BinaryName string `json:"binary_name"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type serverPHPVersionResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Version      types.String `tfsdk:"version"`
	CLIDefault   types.Bool   `tfsdk:"cli_default"`
	SiteDefault  types.Bool   `tfsdk:"site_default"`
	BinaryName   types.String `tfsdk:"binary_name"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *serverPHPVersionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_version"
}

func (r *serverPHPVersionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Installs and manages a PHP version on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to install the PHP version on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "PHP version key to install (e.g. `php82`). Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the installed PHP version.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"cli_default": schema.BoolAttribute{
				MarkdownDescription: "Whether to make this the default CLI PHP version on install. Applied at create time only.",
				Optional:            true,
			},
			"site_default": schema.BoolAttribute{
				MarkdownDescription: "Whether to make this the default site PHP version on install. Applied at create time only.",
				Optional:            true,
			},
			"binary_name": schema.StringAttribute{MarkdownDescription: "PHP binary name (e.g. `php82`).", Computed: true},
			"status":      schema.StringAttribute{MarkdownDescription: "Installation status.", Computed: true},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *serverPHPVersionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPHPVersionResource) basePath(m *serverPHPVersionResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *serverPHPVersionResource) itemPath(m *serverPHPVersionResourceModel) string {
	return fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
}

func (r *serverPHPVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"version": plan.Version.ValueString()}
	if !plan.CLIDefault.IsNull() && !plan.CLIDefault.IsUnknown() {
		body["cli_default"] = plan.CLIDefault.ValueBool()
	}
	if !plan.SiteDefault.IsNull() && !plan.SiteDefault.IsUnknown() {
		body["site_default"] = plan.SiteDefault.ValueBool()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to install Forge PHP version", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected PHP version ID", fmt.Sprintf("Forge returned a non-numeric PHP version ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read PHP version after install", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPVersionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge PHP version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update issues the PUT "update to latest patch release" action. The endpoint
// takes no JSON body; the writable create-time options (cli_default,
// site_default) are not re-applied here.
func (r *serverPHPVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Write(ctx, "PUT", r.itemPath(&plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge PHP version", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read PHP version after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPVersionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverPHPVersionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.itemPath(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to uninstall Forge PHP version", err.Error())
	}
}

// ImportState accepts "organization/server_id/php_version_id".
func (r *serverPHPVersionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/php_version_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	phpVersionID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and php_version_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), phpVersionID)...)
}

// readInto GETs the PHP version identified by m and fills the computed fields.
func (r *serverPHPVersionResource) readInto(ctx context.Context, m *serverPHPVersionResourceModel) error {
	var a serverPHPVersionAttributes
	if _, err := r.client.Get(ctx, r.itemPath(m), &a); err != nil {
		return err
	}
	m.Version = types.StringValue(a.Version)
	m.BinaryName = types.StringValue(a.BinaryName)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}
