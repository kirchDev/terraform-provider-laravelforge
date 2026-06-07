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

// --- Singleton-resource pattern. The PHP max upload size is a single value
// hanging off a server (no separate ID): it is read via GET (show) + replaced
// via PUT (update). Identity is the parent ids only (organization, server_id);
// create == update (PUT), and remove is a no-op (Forge has no destroy endpoint
// for the setting). The PUT body shares UpdatePhpSettingsRequest with the
// max-execution-time / opcache settings, but this resource only owns the
// `max_upload_size` field. ---

var (
	_ resource.Resource                = (*serverPHPMaxUploadSizeResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverPHPMaxUploadSizeResource)(nil)
	_ resource.ResourceWithImportState = (*serverPHPMaxUploadSizeResource)(nil)
)

// NewServerPhpMaxUploadSizeResource returns a new laravelforge_server_php_max_upload_size resource.
func NewServerPhpMaxUploadSizeResource() resource.Resource {
	return &serverPHPMaxUploadSizeResource{}
}

type serverPHPMaxUploadSizeResource struct {
	client *client.Client
}

// serverPHPMaxUploadSizeAttributes mirrors the JSON:API "attributes" of the PHP
// max upload size setting (read shape).
type serverPHPMaxUploadSizeAttributes struct {
	MaxUploadSize *int64 `json:"max_upload_size"`
}

type serverPHPMaxUploadSizeResourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ServerID      types.Int64  `tfsdk:"server_id"`
	MaxUploadSize types.Int64  `tfsdk:"max_upload_size"`
}

func (r *serverPHPMaxUploadSizeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_php_max_upload_size"
}

func (r *serverPHPMaxUploadSizeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the PHP max upload size (`upload_max_filesize` / `post_max_size`) on a Laravel Forge server. " +
			"The setting is a singleton on the server (no separate ID): it is read via `GET` and replaced via `PUT`, " +
			"and removing the resource leaves the current value in place (Forge has no destroy endpoint for it).",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server whose PHP max upload size is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"max_upload_size": schema.Int64Attribute{
				MarkdownDescription: "Maximum upload size in megabytes.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *serverPHPMaxUploadSizeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// settingPath is the singleton path for the server's PHP max upload size.
func (r *serverPHPMaxUploadSizeResource) settingPath(m *serverPHPMaxUploadSizeResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/php/max-upload-size",
		m.Organization.ValueString(), m.ServerID.ValueInt64())
}

// write PUTs the planned max upload size and refreshes it from the response.
func (r *serverPHPMaxUploadSizeResource) write(ctx context.Context, m *serverPHPMaxUploadSizeResourceModel) error {
	body := map[string]any{}
	if !m.MaxUploadSize.IsNull() && !m.MaxUploadSize.IsUnknown() {
		body["max_upload_size"] = m.MaxUploadSize.ValueInt64()
	}

	var a serverPHPMaxUploadSizeAttributes
	if _, err := r.client.Write(ctx, "PUT", r.settingPath(m), body, &a); err != nil {
		return err
	}
	r.apply(m, &a)
	return nil
}

// apply copies response attributes onto the model.
func (r *serverPHPMaxUploadSizeResource) apply(m *serverPHPMaxUploadSizeResourceModel, a *serverPHPMaxUploadSizeAttributes) {
	m.MaxUploadSize = types.Int64PointerValue(a.MaxUploadSize)
}

func (r *serverPHPMaxUploadSizeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverPHPMaxUploadSizeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to set Forge PHP max upload size", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverPHPMaxUploadSizeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverPHPMaxUploadSizeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var a serverPHPMaxUploadSizeAttributes
	if _, err := r.client.Get(ctx, r.settingPath(&state), &a); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge PHP max upload size", err.Error())
		return
	}
	r.apply(&state, &a)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverPHPMaxUploadSizeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverPHPMaxUploadSizeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge PHP max upload size", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the PHP max upload size is a singleton with no destroy
// endpoint, so removing the resource just drops it from state and leaves the
// current value untouched on Forge.
func (r *serverPHPMaxUploadSizeResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/server_id".
func (r *serverPHPMaxUploadSizeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
