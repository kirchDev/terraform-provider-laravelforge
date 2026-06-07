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

// --- Singleton-resource pattern. The php-fpm configuration is a single object
// hanging off a server's PHP version (no separate ID): it is read via GET
// (show) + replaced via PUT (update). Identity is the parent ids only
// (organization, server_id, php_version); create == update (PUT), and remove is
// a no-op (Forge has no destroy endpoint for the fpm config). The PUT body
// field is `config`; the read response echoes it as `configuration`. ---

var (
	_ resource.Resource                = (*serverPHPFpmConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPFpmConfigResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPFpmConfigResource)(nil)
)

// NewServerPHPFpmConfigResource returns a new laravelforge_server_php_fpm_config resource.
func NewServerPHPFpmConfigResource() resource.Resource {
	return &serverPHPFpmConfigResource{}
}

type serverPHPFpmConfigResource struct {
	client *client.Client
}

// serverPHPFpmConfigAttributes mirrors the JSON:API "attributes" of a php-fpm
// configuration (read shape).
type serverPHPFpmConfigAttributes struct {
	Configuration string `json:"configuration"`
}

type serverPHPFpmConfigResourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	PHPVersion    types.String `tfsdk:"php_version"`
	Configuration types.String `tfsdk:"configuration"`
}

func (r *serverPHPFpmConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_fpm_config"
}

func (r *serverPHPFpmConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the php-fpm configuration for a given PHP version on a Laravel Forge server. " +
			"The configuration is a singleton on the server's PHP version (no separate ID): it is read via " +
			"`GET` and replaced via `PUT`, and removing the resource leaves the configuration in place " +
			"(Forge has no destroy endpoint for it).",
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
			"php_version": schema.StringAttribute{
				MarkdownDescription: "PHP version key whose php-fpm configuration is managed (e.g. `php82`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"configuration": schema.StringAttribute{
				MarkdownDescription: "The php-fpm configuration content.",
				Required:            true,
			},
		},
	}
}

func (r *serverPHPFpmConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// configPath is the singleton path for the server PHP version's php-fpm config.
func (r *serverPHPFpmConfigResource) configPath(m *serverPHPFpmConfigResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/versions/%s/configs/fpm",
		m.Organization.ValueString(), m.ServerID.ValueInt64(), m.PHPVersion.ValueString())
}

// write PUTs the planned configuration and refreshes it from the response.
func (r *serverPHPFpmConfigResource) write(ctx context.Context, m *serverPHPFpmConfigResourceModel) error {
	body := map[string]any{"config": m.Configuration.ValueString()}

	var a serverPHPFpmConfigAttributes
	if _, err := r.client.Write(ctx, "PUT", r.configPath(m), body, &a); err != nil {
		return err
	}
	r.apply(m, &a)
	return nil
}

// apply copies response attributes onto the model. configuration stays as
// planned when the API echoes empty (the PUT body is authoritative).
func (r *serverPHPFpmConfigResource) apply(m *serverPHPFpmConfigResourceModel, a *serverPHPFpmConfigAttributes) {
	if a.Configuration != "" {
		m.Configuration = types.StringValue(a.Configuration)
	}
}

func (r *serverPHPFpmConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPFpmConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge php-fpm configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPFpmConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPFpmConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var a serverPHPFpmConfigAttributes
	if _, err := r.client.Get(ctx, r.configPath(&state), &a); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge php-fpm configuration", err.Error())
		return
	}
	r.apply(&state, &a)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPHPFpmConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPFpmConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge php-fpm configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the php-fpm configuration is a singleton with no destroy
// endpoint, so removing the resource just drops it from state and leaves the
// current configuration untouched on Forge.
func (r *serverPHPFpmConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id/php_version".
func (r *serverPHPFpmConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/php_version\".")
		return
	}
	serverID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("php_version"), parts[2])...)
}
