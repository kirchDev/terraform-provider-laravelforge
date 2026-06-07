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

// --- Singleton resource: php.ini CLI config for a given PHP version on a
// server. Keyed by {phpVersion}; GET show + PUT update only, so create == PUT
// and there is no destroy. ---

var (
	_ resource.Resource                = (*serverPHPCLIConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPCLIConfigResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPCLIConfigResource)(nil)
)

// NewServerPhpCliConfigResource returns a new laravelforge_server_php_cli_config resource.
func NewServerPhpCliConfigResource() resource.Resource {
	return &serverPHPCLIConfigResource{}
}

type serverPHPCLIConfigResource struct {
	client *client.Client
}

// serverPHPCLIConfigAttributes mirrors the JSON:API "attributes" of the
// php CLI configuration resource (read shape).
type serverPHPCLIConfigAttributes struct {
	Configuration string `json:"configuration"`
}

type serverPHPCLIConfigModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	PHPVersion    types.Int64  `tfsdk:"php_version"`
	Config        types.String `tfsdk:"config"`
	Configuration types.String `tfsdk:"configuration"`
}

func (r *serverPHPCLIConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_cli_config"
}

func (r *serverPHPCLIConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the php.ini CLI configuration for a given PHP version on a Laravel Forge server. Singleton per PHP version (GET show + PUT update only).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"php_version": schema.Int64Attribute{
				MarkdownDescription: "ID of the PHP version whose CLI config is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "php.ini CLI configuration to write.",
				Required:            true,
			},
			"configuration": schema.StringAttribute{
				MarkdownDescription: "php.ini CLI configuration as reported by Forge.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *serverPHPCLIConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPHPCLIConfigResource) singletonPath(m *serverPHPCLIConfigModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions/%d/configs/cli",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.PHPVersion.ValueInt64())
}

// write PUTs the desired config and reads the resulting state back in.
func (r *serverPHPCLIConfigResource) write(ctx context.Context, m *serverPHPCLIConfigModel) error {
	body := map[string]any{"config": m.Config.ValueString()}
	if _, err := r.client.Write(ctx, "PUT", r.singletonPath(m), body, nil); err != nil {
		return err
	}
	return r.readInto(ctx, m)
}

func (r *serverPHPCLIConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPCLIConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge PHP CLI config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPCLIConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPCLIConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge PHP CLI config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPHPCLIConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPCLIConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge PHP CLI config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the CLI config is a singleton with no destroy endpoint.
func (r *serverPHPCLIConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/php_version".
func (r *serverPHPCLIConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/php_version\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	phpVersion, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and php_version must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("php_version"), phpVersion)...)
}

// readInto GETs the singleton config and fills the computed fields.
func (r *serverPHPCLIConfigResource) readInto(ctx context.Context, m *serverPHPCLIConfigModel) error {
	var a serverPHPCLIConfigAttributes
	if _, err := r.client.Get(ctx, r.singletonPath(m), &a); err != nil {
		return err
	}
	m.Configuration = types.StringValue(a.Configuration)
	return nil
}
