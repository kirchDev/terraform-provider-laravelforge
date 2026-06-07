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

// Singleton resource: the default CLI PHP version of a server. There is no own
// id — identity is the parent ids (organization/server_id). The GET show
// confirms existence and returns the active version; Create and Update both PUT
// the singleton path (create == PUT). Delete is a no-op (the server always has
// some CLI version; there is nothing to destroy). It selects which installed
// PHP version the `php` CLI points to.

var (
	_ resource.Resource                = (*serverPHPCLIVersionResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPCLIVersionResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPCLIVersionResource)(nil)
)

// NewServerPHPCLIVersionResource returns a new laravelforge_server_php_cli_version resource.
func NewServerPHPCLIVersionResource() resource.Resource {
	return &serverPHPCLIVersionResource{}
}

type serverPHPCLIVersionResource struct {
	client *client.Client
}

// serverPHPCLIVersionAttributes mirrors the JSON:API "attributes" of a PHP
// version resource (read shape, PhpVersionResource). Shared with the data source.
type serverPHPCLIVersionAttributes struct {
	Version    string `json:"version"`
	BinaryName string `json:"binary_name"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type serverPHPCLIVersionResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	PHPVersion   types.String `tfsdk:"php_version"`
	Version      types.String `tfsdk:"version"`
	BinaryName   types.String `tfsdk:"binary_name"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *serverPHPCLIVersionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_cli_version"
}

func (r *serverPHPCLIVersionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the default CLI PHP version of a Laravel Forge server (singleton). Selects which installed PHP version the `php` CLI points to.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server whose CLI PHP version is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"php_version": schema.StringAttribute{
				MarkdownDescription: "PHP version the CLI should point to. One of `5.6`, `7.0`, `7.1`, `7.2`, `7.3`, `7.4`, `8.0`, `8.1`, `8.2`, `8.3`, `8.4`, `8.5`.",
				Required:            true,
			},
			"version":     schema.StringAttribute{MarkdownDescription: "Active CLI PHP version reported by Forge.", Computed: true},
			"binary_name": schema.StringAttribute{MarkdownDescription: "Name of the PHP binary (e.g. `php82`).", Computed: true},
			"status":      schema.StringAttribute{MarkdownDescription: "Installation status of the PHP version.", Computed: true},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *serverPHPCLIVersionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverPHPCLIVersionResource) singletonPath(m *serverPHPCLIVersionResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/cli-version", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

// put sends the desired CLI PHP version. create == update for a singleton.
func (r *serverPHPCLIVersionResource) put(ctx context.Context, m *serverPHPCLIVersionResourceModel) error {
	body := map[string]any{"php_version": m.PHPVersion.ValueString()}
	_, err := r.client.Write(ctx, "PUT", r.singletonPath(m), body, nil)
	return err
}

// readInto GETs the singleton and fills the computed fields.
func (r *serverPHPCLIVersionResource) readInto(ctx context.Context, m *serverPHPCLIVersionResourceModel) error {
	var a serverPHPCLIVersionAttributes
	if _, err := r.client.Get(ctx, r.singletonPath(m), &a); err != nil {
		return err
	}
	m.Version = types.StringValue(a.Version)
	m.BinaryName = types.StringValue(a.BinaryName)
	m.Status = types.StringValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}

func (r *serverPHPCLIVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPCLIVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge CLI PHP version", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read CLI PHP version after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPCLIVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPCLIVersionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge CLI PHP version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPHPCLIVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPCLIVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge CLI PHP version", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read CLI PHP version after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the server always has a default CLI PHP version; there is
// no destroy endpoint and nothing to remove.
func (r *serverPHPCLIVersionResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id". php_version is read back from
// the API on the subsequent refresh.
func (r *serverPHPCLIVersionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
