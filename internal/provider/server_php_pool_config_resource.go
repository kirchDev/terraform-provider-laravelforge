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

// --- Singleton-resource pattern: the php-fpm pool config for a given php
// version on a server (one per phpVersion). No own id; identity is the parent
// ids (organization/server_id/php_version_id). Read=GET the pool config path,
// Create==Update=PUT it, Delete is a no-op (the pool config always exists for
// a php version; removing the resource just stops Terraform managing it). ---

var (
	_ resource.Resource                = (*serverPhpPoolConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPhpPoolConfigResource)(nil)
	_ resource.ResourceWithImportState = (*serverPhpPoolConfigResource)(nil)
)

// NewServerPhpPoolConfigResource returns a new laravelforge_server_php_pool_config resource.
func NewServerPhpPoolConfigResource() resource.Resource {
	return &serverPhpPoolConfigResource{}
}

type serverPhpPoolConfigResource struct {
	client *client.Client
}

// serverPhpPoolConfigAttributes mirrors the JSON:API "attributes" of the pool
// config resource (read shape): the rendered php-fpm pool configuration.
type serverPhpPoolConfigAttributes struct {
	Configuration string `json:"configuration"`
}

type serverPhpPoolConfigResourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	PHPVersionID  types.Int64  `tfsdk:"php_version_id"`
	Config        types.String `tfsdk:"config"`
	User          types.String `tfsdk:"user"`
	Configuration types.String `tfsdk:"configuration"`
}

func (r *serverPhpPoolConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_pool_config"
}

func (r *serverPhpPoolConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the php-fpm pool configuration of a PHP version on a Laravel Forge server (singleton per PHP version).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the PHP version belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"php_version_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the PHP version whose pool config is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "The php-fpm pool configuration contents to write.",
				Required:            true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "Optional pool user to scope the configuration to.",
				Optional:            true,
			},
			"configuration": schema.StringAttribute{
				MarkdownDescription: "The current php-fpm pool configuration as returned by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *serverPhpPoolConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPhpPoolConfigResource) path(m *serverPhpPoolConfigResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions/%d/configs/pool", m.Organization.ValueString(), m.ServerID.ValueInt64(), m.PHPVersionID.ValueInt64())
}

// write PUTs the planned pool config (Create and Update share this).
func (r *serverPhpPoolConfigResource) write(ctx context.Context, m *serverPhpPoolConfigResourceModel) error {
	body := map[string]any{"config": m.Config.ValueString()}
	if !m.User.IsNull() && !m.User.IsUnknown() {
		body["user"] = m.User.ValueString()
	}
	_, err := r.client.Write(ctx, "PUT", r.path(m), body, nil)
	return err
}

func (r *serverPhpPoolConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPhpPoolConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to create Forge PHP pool config", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read PHP pool config after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPhpPoolConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPhpPoolConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge PHP pool config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPhpPoolConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPhpPoolConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge PHP pool config", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read PHP pool config after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: a PHP version's pool config always exists, so there is
// nothing to destroy. Removing the resource just stops Terraform managing it.
func (r *serverPhpPoolConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/php_version_id".
func (r *serverPhpPoolConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("php_version_id"), phpVersionID)...)
}

// readInto GETs the pool config singleton identified by m and fills configuration.
func (r *serverPhpPoolConfigResource) readInto(ctx context.Context, m *serverPhpPoolConfigResourceModel) error {
	var a serverPhpPoolConfigAttributes
	if _, err := r.client.Get(ctx, r.path(m), &a); err != nil {
		return err
	}
	m.Configuration = types.StringValue(a.Configuration)
	return nil
}
