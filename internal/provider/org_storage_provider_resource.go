package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ resource.Resource                = (*orgStorageProviderResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgStorageProviderResource)(nil)
	_ resource.ResourceWithImportState = (*orgStorageProviderResource)(nil)
)

// NewOrgStorageProviderResource returns a new laravelforge_org_storage_provider resource.
func NewOrgStorageProviderResource() resource.Resource {
	return &orgStorageProviderResource{}
}

type orgStorageProviderResource struct {
	client *client.Client
}

// orgStorageProviderAttributes mirrors the JSON:API "attributes" of a storage
// provider (read shape). Credentials (access_key/secret_key) are never returned.
type orgStorageProviderAttributes struct {
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	ProviderName string  `json:"provider_name"`
	Region       *string `json:"region"`
	Bucket       *string `json:"bucket"`
	Directory    *string `json:"directory"`
	Endpoint     *string `json:"endpoint"`
	AssumeRole   *bool   `json:"assume_role"`
	InUse        bool    `json:"in_use"`
	CreatedAt    *string `json:"created_at"`
	UpdatedAt    *string `json:"updated_at"`
}

type orgStorageProviderResourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ID            types.Int64  `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	ProviderName  types.String `tfsdk:"provider_name"`
	Region        types.String `tfsdk:"region"`
	Bucket        types.String `tfsdk:"bucket"`
	Directory     types.String `tfsdk:"directory"`
	Endpoint      types.String `tfsdk:"endpoint"`
	AccessKey     types.String `tfsdk:"access_key"`
	SecretKey     types.String `tfsdk:"secret_key"`
	AssumeRole    types.Bool   `tfsdk:"assume_role"`
	InUse         types.Bool   `tfsdk:"in_use"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *orgStorageProviderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_storage_provider"
}

func (r *orgStorageProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an organization storage provider (backup destination, e.g. S3) on Laravel Forge.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the storage provider.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the storage provider.",
				Required:            true,
			},
			"cloud_provider": schema.StringAttribute{
				MarkdownDescription: "Storage provider type (`s3`, `spaces`, `hetzner`, `ovh`, `scaleway`, `custom`).",
				Required:            true,
			},
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable provider name (e.g. `Amazon S3`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Storage region.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bucket": schema.StringAttribute{
				MarkdownDescription: "Bucket name.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"directory": schema.StringAttribute{
				MarkdownDescription: "Directory (path prefix) within the bucket.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Custom S3-compatible endpoint URL.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"access_key": schema.StringAttribute{
				MarkdownDescription: "Access key for the storage provider. Write-only; never returned by the API.",
				Optional:            true,
				Sensitive:           true,
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "Secret key for the storage provider. Write-only; never returned by the API.",
				Optional:            true,
				Sensitive:           true,
			},
			"assume_role": schema.BoolAttribute{
				MarkdownDescription: "Whether to use an EC2 assumed IAM role instead of access/secret keys.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"in_use": schema.BoolAttribute{
				MarkdownDescription: "Whether the storage provider is currently used by a backup configuration.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (r *orgStorageProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *orgStorageProviderResource) basePath(m *orgStorageProviderResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/storage-providers", m.Organization.ValueString())
}

func (r *orgStorageProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgStorageProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"provider": plan.CloudProvider.ValueString(),
	}
	if !plan.Region.IsNull() && !plan.Region.IsUnknown() {
		body["region"] = plan.Region.ValueString()
	}
	if !plan.Bucket.IsNull() && !plan.Bucket.IsUnknown() {
		body["bucket"] = plan.Bucket.ValueString()
	}
	if !plan.Directory.IsNull() && !plan.Directory.IsUnknown() {
		body["directory"] = plan.Directory.ValueString()
	}
	if !plan.Endpoint.IsNull() && !plan.Endpoint.IsUnknown() {
		body["endpoint"] = plan.Endpoint.ValueString()
	}
	if !plan.AccessKey.IsNull() && !plan.AccessKey.IsUnknown() {
		body["access_key"] = plan.AccessKey.ValueString()
	}
	if !plan.SecretKey.IsNull() && !plan.SecretKey.IsUnknown() {
		body["secret_key"] = plan.SecretKey.ValueString()
	}
	if !plan.AssumeRole.IsNull() && !plan.AssumeRole.IsUnknown() {
		body["assume_role"] = plan.AssumeRole.ValueBool()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge storage provider", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected storage provider ID", fmt.Sprintf("Forge returned a non-numeric ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read storage provider after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgStorageProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgStorageProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge storage provider", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *orgStorageProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgStorageProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"provider": plan.CloudProvider.ValueString(),
	}
	if !plan.Region.IsNull() && !plan.Region.IsUnknown() {
		body["region"] = plan.Region.ValueString()
	}
	if !plan.Bucket.IsNull() && !plan.Bucket.IsUnknown() {
		body["bucket"] = plan.Bucket.ValueString()
	}
	if !plan.Directory.IsNull() && !plan.Directory.IsUnknown() {
		body["directory"] = plan.Directory.ValueString()
	}
	if !plan.Endpoint.IsNull() && !plan.Endpoint.IsUnknown() {
		body["endpoint"] = plan.Endpoint.ValueString()
	}
	if !plan.AccessKey.IsNull() && !plan.AccessKey.IsUnknown() {
		body["access_key"] = plan.AccessKey.ValueString()
	}
	if !plan.SecretKey.IsNull() && !plan.SecretKey.IsUnknown() {
		body["secret_key"] = plan.SecretKey.ValueString()
	}
	if !plan.AssumeRole.IsNull() && !plan.AssumeRole.IsUnknown() {
		// Update body names the assume-role toggle use_ec2_assumed_role.
		body["use_ec2_assumed_role"] = plan.AssumeRole.ValueBool()
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge storage provider", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read storage provider after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgStorageProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgStorageProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge storage provider", err.Error())
	}
}

// ImportState accepts "organization/id". Credentials are not importable.
func (r *orgStorageProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// readInto GETs the storage provider identified by m and fills the
// computed/optional fields. Credentials are never returned, so access_key and
// secret_key are left untouched (preserving config/state values).
func (r *orgStorageProviderResource) readInto(ctx context.Context, m *orgStorageProviderResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a orgStorageProviderAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.CloudProvider = types.StringValue(a.Provider)
	m.ProviderName = types.StringValue(a.ProviderName)
	m.Region = types.StringPointerValue(a.Region)
	m.Bucket = types.StringPointerValue(a.Bucket)
	m.Directory = types.StringPointerValue(a.Directory)
	m.Endpoint = types.StringPointerValue(a.Endpoint)
	m.AssumeRole = types.BoolPointerValue(a.AssumeRole)
	m.InUse = types.BoolValue(a.InUse)
	m.CreatedAt = types.StringPointerValue(a.CreatedAt)
	m.UpdatedAt = types.StringPointerValue(a.UpdatedAt)
	return nil
}
