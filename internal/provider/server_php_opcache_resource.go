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

// --- Singleton resource: server PHP OPcache enable/disable. ---
//
// There is no PUT/PATCH on this endpoint, so it is not configurable: presence
// (the POST having run) means OPcache is enabled. The only attribute is the
// computed boolean opcache_enabled returned by the show endpoint. Identity is
// the parent ids only (organization + server_id); there is no own id.

var (
	_ resource.Resource                = (*serverPHPOpcacheResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPOpcacheResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPOpcacheResource)(nil)
)

// NewServerPhpOpcacheResource returns a new laravelforge_server_php_opcache resource.
func NewServerPhpOpcacheResource() resource.Resource {
	return &serverPHPOpcacheResource{}
}

type serverPHPOpcacheResource struct {
	client *client.Client
}

// serverPHPOpcacheAttributes mirrors the JSON:API "attributes" of the
// PhpOpcacheResource (read shape).
type serverPHPOpcacheAttributes struct {
	OpcacheEnabled bool `json:"opcache_enabled"`
}

type serverPHPOpcacheModel struct {
	Organization   types.String `tfsdk:"organization"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	OpcacheEnabled types.Bool   `tfsdk:"opcache_enabled"`
}

func (r *serverPHPOpcacheResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_opcache"
}

func (r *serverPHPOpcacheResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables PHP OPcache on a Laravel Forge server. This is a singleton toggle: creating the resource enables OPcache, destroying it disables OPcache. There is no in-place update.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to enable PHP OPcache on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"opcache_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether PHP OPcache is enabled on the server.",
				Computed:            true,
			},
		},
	}
}

func (r *serverPHPOpcacheResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPHPOpcacheResource) path(m *serverPHPOpcacheModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/opcache", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *serverPHPOpcacheResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPOpcacheModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// POST enables OPcache; there is no JSON body.
	if _, err := r.client.Write(ctx, "POST", r.path(&plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to enable Forge PHP OPcache", err.Error())
		return
	}

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read PHP OPcache after enable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPOpcacheResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPOpcacheModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge PHP OPcache", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update can never run: both inputs are RequiresReplace and the only other
// attribute is computed. Defined to satisfy the resource.Resource interface.
func (r *serverPHPOpcacheResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPOpcacheModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge PHP OPcache", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPOpcacheResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverPHPOpcacheModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Forge PHP OPcache", err.Error())
	}
}

// ImportState accepts "organization/server_id".
func (r *serverPHPOpcacheResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the OPcache status for the server identified by m and fills the
// computed attribute.
func (r *serverPHPOpcacheResource) readInto(ctx context.Context, m *serverPHPOpcacheModel) error {
	var a serverPHPOpcacheAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.OpcacheEnabled = types.BoolValue(a.OpcacheEnabled)
	return nil
}
