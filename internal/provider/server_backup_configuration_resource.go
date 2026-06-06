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

// --- CRUD-resource pattern, following site_resource.go / database_resource.go. ---
//
// A Forge "backup configuration" is the schedule/retention/storage-provider
// config for database backups on a server. Verified against the OpenAPI spec:
//   - list/create:  GET/POST   /api/orgs/{org}/servers/{server}/database/backups
//   - read/update/delete: GET / PUT / DELETE  .../database/backups/{id}
//     (server-scoped item path, like database/schemas).
//   - Create/Update bodies are FLAT and identical (Create*/Update*Request share
//     the same shape). Required create inputs: storage_provider_id, frequency,
//     retention, database_ids.
//   - database_ids is a required ARRAY input (the only non-scalar this resource
//     can't avoid); it's an input-only field here and is not round-tripped into
//     state (the read response returns it as a free-form array). All other
//     mapped attributes are scalars.
//   - The per-instance trigger (.../instances) and restore are excluded
//     (actions); instance listing lives in the data source.

var (
	_ resource.Resource                = (*serverBackupConfigurationResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverBackupConfigurationResource)(nil)
	_ resource.ResourceWithImportState = (*serverBackupConfigurationResource)(nil)
)

// NewServerBackupConfigurationResource returns a new
// laravelforge_server_backup_configuration resource.
func NewServerBackupConfigurationResource() resource.Resource {
	return &serverBackupConfigurationResource{}
}

type serverBackupConfigurationResource struct {
	client *client.Client
}

// serverBackupConfigurationAttributes mirrors the JSON:API "attributes" of a
// backup configuration (read shape). Only scalar fields are mapped.
type serverBackupConfigurationAttributes struct {
	Name                string  `json:"name"`
	StorageProviderID   *int64  `json:"storage_provider_id"`
	Provider            string  `json:"provider"`
	Bucket              *string `json:"bucket"`
	Directory           string  `json:"directory"`
	Schedule            string  `json:"schedule"`
	DisplayableSchedule string  `json:"displayable_schedule"`
	NextRunTime         string  `json:"next_run_time"`
	Status              string  `json:"status"`
	DayOfWeek           *int64  `json:"day_of_week"`
	Time                *string `json:"time"`
	CronSchedule        *string `json:"cron_schedule"`
	Retention           int64   `json:"retention"`
	NotifyEmail         *string `json:"notify_email"`
}

type serverBackupConfigurationResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`

	// Write-only / input fields (FLAT create/update body).
	StorageProviderID types.Int64  `tfsdk:"storage_provider_id"`
	Frequency         types.String `tfsdk:"frequency"`
	Day               types.String `tfsdk:"day"`
	Cron              types.String `tfsdk:"cron"`
	DatabaseIDs       types.List   `tfsdk:"database_ids"`
	NotificationEmail types.String `tfsdk:"notification_email"`

	// Read/computed scalar attributes.
	Name                types.String `tfsdk:"name"`
	CloudProvider       types.String `tfsdk:"cloud_provider"`
	Bucket              types.String `tfsdk:"bucket"`
	Directory           types.String `tfsdk:"directory"`
	Schedule            types.String `tfsdk:"schedule"`
	DisplayableSchedule types.String `tfsdk:"displayable_schedule"`
	NextRunTime         types.String `tfsdk:"next_run_time"`
	Status              types.String `tfsdk:"status"`
	DayOfWeek           types.Int64  `tfsdk:"day_of_week"`
	Time                types.String `tfsdk:"time"`
	CronSchedule        types.String `tfsdk:"cron_schedule"`
	Retention           types.Int64  `tfsdk:"retention"`
	NotifyEmail         types.String `tfsdk:"notify_email"`
}

func (r *serverBackupConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_backup_configuration"
}

func (r *serverBackupConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a database backup configuration (schedule, retention, storage provider) on a Laravel Forge server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server the backup configuration belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the backup configuration.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},

			// --- Write inputs (FLAT create/update body). ---
			"storage_provider_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the storage provider (credential) the backups are written to.",
				Required:            true,
			},
			"frequency": schema.StringAttribute{
				MarkdownDescription: "Backup frequency: `hourly`, `daily`, `weekly`, or `custom`.",
				Required:            true,
			},
			"day": schema.StringAttribute{
				MarkdownDescription: "Day of week for weekly backups (`0`-`6`). Write-only input.",
				Optional:            true,
			},
			"cron": schema.StringAttribute{
				MarkdownDescription: "Custom cron expression (used when `frequency` is `custom`). Write-only input.",
				Optional:            true,
			},
			"database_ids": schema.ListAttribute{
				MarkdownDescription: "IDs of the databases to back up. Write-only input (not round-tripped into state).",
				Required:            true,
				ElementType:         types.Int64Type,
			},
			"notification_email": schema.StringAttribute{
				MarkdownDescription: "Email address notified on backup events. Write-only input; the API reports it back as `notify_email`.",
				Optional:            true,
			},

			// --- Read / computed scalar attributes. ---
			"name":                 schema.StringAttribute{MarkdownDescription: "Name of the backup configuration.", Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"bucket":               schema.StringAttribute{MarkdownDescription: "Storage bucket the backups are written to.", Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"directory":            schema.StringAttribute{MarkdownDescription: "Directory within the bucket.", Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"time":                 schema.StringAttribute{MarkdownDescription: "Time of day the backup runs (e.g. `03:00`).", Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"retention":            schema.Int64Attribute{MarkdownDescription: "Number of backups to retain.", Required: true},
			"cloud_provider":       schema.StringAttribute{MarkdownDescription: "Underlying storage provider (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
			"schedule":             schema.StringAttribute{MarkdownDescription: "Resolved cron schedule.", Computed: true},
			"displayable_schedule": schema.StringAttribute{MarkdownDescription: "Human-readable schedule.", Computed: true},
			"next_run_time":        schema.StringAttribute{MarkdownDescription: "Timestamp of the next scheduled run.", Computed: true},
			"status":               schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"day_of_week":          schema.Int64Attribute{MarkdownDescription: "Day of week the backup runs (`0`-`6`), if weekly.", Computed: true},
			"cron_schedule":        schema.StringAttribute{MarkdownDescription: "Custom cron schedule, if `frequency` is `custom`.", Computed: true},
			"notify_email":         schema.StringAttribute{MarkdownDescription: "Email address notified on backup events.", Computed: true},
		},
	}
}

func (r *serverBackupConfigurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverBackupConfigurationResource) basePath(m *serverBackupConfigurationResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/database/backups", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

// writeBody builds the FLAT create/update request body. Create and Update share
// the same shape (CreateBackupConfigurationRequest == UpdateBackupConfigurationRequest).
func (r *serverBackupConfigurationResource) writeBody(ctx context.Context, m *serverBackupConfigurationResourceModel) (map[string]any, error) {
	body := map[string]any{
		"storage_provider_id": m.StorageProviderID.ValueInt64(),
		"frequency":           m.Frequency.ValueString(),
		"retention":           m.Retention.ValueInt64(),
	}

	ids := make([]int64, 0, len(m.DatabaseIDs.Elements()))
	if diags := m.DatabaseIDs.ElementsAs(ctx, &ids, false); diags.HasError() {
		return nil, fmt.Errorf("invalid database_ids")
	}
	body["database_ids"] = ids

	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		body["name"] = m.Name.ValueString()
	}
	if !m.Bucket.IsNull() && !m.Bucket.IsUnknown() {
		body["bucket"] = m.Bucket.ValueString()
	}
	if !m.Directory.IsNull() && !m.Directory.IsUnknown() {
		body["directory"] = m.Directory.ValueString()
	}
	if !m.Day.IsNull() && !m.Day.IsUnknown() {
		body["day"] = m.Day.ValueString()
	}
	if !m.Time.IsNull() && !m.Time.IsUnknown() {
		body["time"] = m.Time.ValueString()
	}
	if !m.Cron.IsNull() && !m.Cron.IsUnknown() {
		body["cron"] = m.Cron.ValueString()
	}
	if !m.NotificationEmail.IsNull() && !m.NotificationEmail.IsUnknown() {
		body["notification_email"] = m.NotificationEmail.ValueString()
	}
	return body, nil
}

func (r *serverBackupConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverBackupConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := r.writeBody(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid backup configuration plan", err.Error())
		return
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge backup configuration", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected backup configuration ID", fmt.Sprintf("Forge returned a non-numeric backup configuration ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read backup configuration after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverBackupConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverBackupConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge backup configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverBackupConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverBackupConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := r.writeBody(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid backup configuration plan", err.Error())
		return
	}

	itemPath := fmt.Sprintf("%s/%d", r.basePath(&plan), plan.ID.ValueInt64())
	if _, err := r.client.Write(ctx, "PUT", itemPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update Forge backup configuration", err.Error())
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read backup configuration after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverBackupConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverBackupConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge backup configuration", err.Error())
	}
}

// ImportState accepts "organization/server_id/backup_configuration_id".
func (r *serverBackupConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/backup_configuration_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	backupID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and backup_configuration_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), backupID)...)
}

// readInto GETs the backup configuration identified by m and fills the
// computed/optional scalar fields. The single-resource read stays server-scoped
// (per the resource's links.self, like database/schemas).
func (r *serverBackupConfigurationResource) readInto(ctx context.Context, m *serverBackupConfigurationResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a serverBackupConfigurationAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.StorageProviderID = types.Int64PointerValue(a.StorageProviderID)
	m.CloudProvider = types.StringValue(a.Provider)
	m.Bucket = types.StringPointerValue(a.Bucket)
	m.Directory = types.StringValue(a.Directory)
	m.Schedule = types.StringValue(a.Schedule)
	m.DisplayableSchedule = types.StringValue(a.DisplayableSchedule)
	m.NextRunTime = types.StringValue(a.NextRunTime)
	m.Status = types.StringValue(a.Status)
	m.DayOfWeek = types.Int64PointerValue(a.DayOfWeek)
	m.Time = types.StringPointerValue(a.Time)
	m.CronSchedule = types.StringPointerValue(a.CronSchedule)
	m.Retention = types.Int64Value(a.Retention)
	m.NotifyEmail = types.StringPointerValue(a.NotifyEmail)
	return nil
}
