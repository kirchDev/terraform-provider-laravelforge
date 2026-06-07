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

// --- Singleton resource: default site/FPM PHP version for a Laravel Forge
// server. Identity is the parent ids only (organization + server_id); GET show
// + PUT update only, so create == PUT and there is no destroy. ---

var (
	_ resource.Resource                = (*serverPHPSiteVersionResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPSiteVersionResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPSiteVersionResource)(nil)
)

// NewServerPhpSiteVersionResource returns a new laravelforge_server_php_site_version resource.
func NewServerPhpSiteVersionResource() resource.Resource {
	return &serverPHPSiteVersionResource{}
}

type serverPHPSiteVersionResource struct {
	client *client.Client
}

// serverPHPSiteVersionAttributes mirrors the JSON:API "attributes" of the
// PhpVersionResource (read shape).
type serverPHPSiteVersionAttributes struct {
	Version    string `json:"version"`
	BinaryName string `json:"binary_name"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type serverPHPSiteVersionModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	PHPVersion   types.String `tfsdk:"php_version"`
	Version      types.String `tfsdk:"version"`
	BinaryName   types.String `tfsdk:"binary_name"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *serverPHPSiteVersionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_site_version"
}

func (r *serverPHPSiteVersionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the default site/FPM PHP version for a Laravel Forge server. Singleton per server (GET show + PUT update only).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server whose default site PHP version is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"php_version": schema.StringAttribute{
				MarkdownDescription: "PHP version to set as the server's default for sites (e.g. `8.2`).",
				Required:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "PHP version as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"binary_name": schema.StringAttribute{MarkdownDescription: "PHP binary name (e.g. `php82`).", Computed: true},
			"status":      schema.StringAttribute{MarkdownDescription: "PHP version status.", Computed: true},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *serverPHPSiteVersionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPHPSiteVersionResource) singletonPath(m *serverPHPSiteVersionModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/site-version",
		m.Organization.ValueString(), m.ServerID.ValueInt64())
}

// write PUTs the desired PHP version and reads the resulting state back in.
func (r *serverPHPSiteVersionResource) write(ctx context.Context, m *serverPHPSiteVersionModel) error {
	body := map[string]any{"php_version": m.PHPVersion.ValueString()}
	if _, err := r.client.Write(ctx, "PUT", r.singletonPath(m), body, nil); err != nil {
		return err
	}
	return r.readInto(ctx, m)
}

func (r *serverPHPSiteVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPSiteVersionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge default site PHP version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPSiteVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPSiteVersionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge default site PHP version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPHPSiteVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPSiteVersionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge default site PHP version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the default site PHP version is a singleton with no
// destroy endpoint.
func (r *serverPHPSiteVersionResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id".
func (r *serverPHPSiteVersionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id\".")
		return
	}
	serverID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
}

// readInto GETs the singleton default site PHP version and fills the computed
// fields. The reported "version" is mirrored into php_version so plan and state
// converge.
func (r *serverPHPSiteVersionResource) readInto(ctx context.Context, m *serverPHPSiteVersionModel) error {
	var a serverPHPSiteVersionAttributes
	if _, err := r.client.Get(ctx, r.singletonPath(m), &a); err != nil {
		return err
	}
	m.PHPVersion = types.StringValue(a.Version)
	m.Version = types.StringValue(a.Version)
	m.BinaryName = types.StringValue(a.BinaryName)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}
