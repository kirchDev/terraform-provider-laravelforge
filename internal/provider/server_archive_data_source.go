package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ datasource.DataSource              = (*serverArchiveDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverArchiveDataSource)(nil)
)

// NewServerArchiveDataSource returns a new laravelforge_server_archive data
// source. Forge only exposes an index endpoint for archived (deleted) servers
// (the per-id path is delete-only, used to purge), so this lists every
// archived server in the organization.
func NewServerArchiveDataSource() datasource.DataSource {
	return &serverArchiveDataSource{}
}

type serverArchiveDataSource struct {
	client *client.Client
}

// serverArchiveAttributes mirrors the scalar JSON:API "attributes" of an
// archived server (ServerResource). Object/array fields (relationships, links)
// are intentionally omitted.
type serverArchiveAttributes struct {
	ID               int64   `json:"id"`
	CredentialID     *int64  `json:"credential_id"`
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Type             string  `json:"type"`
	UbuntuVersion    *string `json:"ubuntu_version"`
	SSHPort          int64   `json:"ssh_port"`
	Provider         string  `json:"provider"`
	Identifier       *string `json:"identifier"`
	Size             string  `json:"size"`
	Region           string  `json:"region"`
	PHPVersion       *string `json:"php_version"`
	PHPCLIVersion    *string `json:"php_cli_version"`
	OpcacheStatus    *string `json:"opcache_status"`
	DatabaseType     *string `json:"database_type"`
	DBStatus         *string `json:"db_status"`
	RedisStatus      *string `json:"redis_status"`
	IPAddress        *string `json:"ip_address"`
	PrivateIPAddress *string `json:"private_ip_address"`
	Revoked          bool    `json:"revoked"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	ConnectionStatus string  `json:"connection_status"`
	Timezone         string  `json:"timezone"`
	LocalPublicKey   *string `json:"local_public_key"`
	IsReady          bool    `json:"is_ready"`
}

// serverArchiveModel is one archived server in the datasource's list output.
type serverArchiveModel struct {
	ID               types.Int64  `tfsdk:"id"`
	CredentialID     types.Int64  `tfsdk:"credential_id"`
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	Type             types.String `tfsdk:"type"`
	UbuntuVersion    types.String `tfsdk:"ubuntu_version"`
	SSHPort          types.Int64  `tfsdk:"ssh_port"`
	CloudProvider    types.String `tfsdk:"cloud_provider"`
	Identifier       types.String `tfsdk:"identifier"`
	Size             types.String `tfsdk:"size"`
	Region           types.String `tfsdk:"region"`
	PHPVersion       types.String `tfsdk:"php_version"`
	PHPCLIVersion    types.String `tfsdk:"php_cli_version"`
	OpcacheStatus    types.String `tfsdk:"opcache_status"`
	DatabaseType     types.String `tfsdk:"database_type"`
	DBStatus         types.String `tfsdk:"db_status"`
	RedisStatus      types.String `tfsdk:"redis_status"`
	IPAddress        types.String `tfsdk:"ip_address"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	Revoked          types.Bool   `tfsdk:"revoked"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	ConnectionStatus types.String `tfsdk:"connection_status"`
	Timezone         types.String `tfsdk:"timezone"`
	LocalPublicKey   types.String `tfsdk:"local_public_key"`
	IsReady          types.Bool   `tfsdk:"is_ready"`
}

type serverArchiveDataSourceModel struct {
	Organization types.String         `tfsdk:"organization"`
	Servers      []serverArchiveModel `tfsdk:"servers"`
}

func (d *serverArchiveDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_archive"
}

func (d *serverArchiveDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all archived (deleted) Laravel Forge servers in an organization. Forge only exposes an index endpoint for archived servers, so there is no single-server lookup; filter the `servers` list by `id` in HCL instead.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization whose archived servers to list.", Required: true},
			"servers": schema.ListNestedAttribute{
				MarkdownDescription: "Archived servers belonging to the organization.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                 schema.Int64Attribute{MarkdownDescription: "Numeric ID of the archived server.", Computed: true},
						"credential_id":      schema.Int64Attribute{MarkdownDescription: "ID of the provider credential the server was created with, if any.", Computed: true},
						"name":               schema.StringAttribute{MarkdownDescription: "Server name.", Computed: true},
						"slug":               schema.StringAttribute{MarkdownDescription: "Server slug.", Computed: true},
						"type":               schema.StringAttribute{MarkdownDescription: "Server type (e.g. `app`, `web`, `database`).", Computed: true},
						"ubuntu_version":     schema.StringAttribute{MarkdownDescription: "Ubuntu version the server ran.", Computed: true},
						"ssh_port":           schema.Int64Attribute{MarkdownDescription: "SSH port.", Computed: true},
						"cloud_provider":     schema.StringAttribute{MarkdownDescription: "Underlying server provider (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
						"identifier":         schema.StringAttribute{MarkdownDescription: "Provider-side identifier of the server.", Computed: true},
						"size":               schema.StringAttribute{MarkdownDescription: "Server size / plan.", Computed: true},
						"region":             schema.StringAttribute{MarkdownDescription: "Region the server ran in.", Computed: true},
						"php_version":        schema.StringAttribute{MarkdownDescription: "Installed PHP version.", Computed: true},
						"php_cli_version":    schema.StringAttribute{MarkdownDescription: "Installed PHP CLI version.", Computed: true},
						"opcache_status":     schema.StringAttribute{MarkdownDescription: "OPcache status.", Computed: true},
						"database_type":      schema.StringAttribute{MarkdownDescription: "Installed database type.", Computed: true},
						"db_status":          schema.StringAttribute{MarkdownDescription: "Database installation status.", Computed: true},
						"redis_status":       schema.StringAttribute{MarkdownDescription: "Redis installation status.", Computed: true},
						"ip_address":         schema.StringAttribute{MarkdownDescription: "Public IP address.", Computed: true},
						"private_ip_address": schema.StringAttribute{MarkdownDescription: "Private IP address.", Computed: true},
						"revoked":            schema.BoolAttribute{MarkdownDescription: "Whether the server's access has been revoked.", Computed: true},
						"created_at":         schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
						"updated_at":         schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
						"connection_status":  schema.StringAttribute{MarkdownDescription: "Connection status.", Computed: true},
						"timezone":           schema.StringAttribute{MarkdownDescription: "Server timezone.", Computed: true},
						"local_public_key":   schema.StringAttribute{MarkdownDescription: "Server's local public SSH key.", Computed: true},
						"is_ready":           schema.BoolAttribute{MarkdownDescription: "Whether the server had finished provisioning.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *serverArchiveDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = c
}

func (d *serverArchiveDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverArchiveDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/archives", data.Organization.ValueString())
	raw, err := d.client.List(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list Forge archived servers", err.Error())
		return
	}

	data.Servers = make([]serverArchiveModel, 0, len(raw))
	for _, r := range raw {
		var a serverArchiveAttributes
		if len(r.Attributes) > 0 {
			if err := json.Unmarshal(r.Attributes, &a); err != nil {
				resp.Diagnostics.AddError("Unable to decode Forge archived server", err.Error())
				return
			}
		}
		data.Servers = append(data.Servers, serverArchiveModel{
			ID:               types.Int64Value(a.ID),
			CredentialID:     types.Int64PointerValue(a.CredentialID),
			Name:             types.StringValue(a.Name),
			Slug:             types.StringValue(a.Slug),
			Type:             types.StringValue(a.Type),
			UbuntuVersion:    types.StringPointerValue(a.UbuntuVersion),
			SSHPort:          types.Int64Value(a.SSHPort),
			CloudProvider:    types.StringValue(a.Provider),
			Identifier:       types.StringPointerValue(a.Identifier),
			Size:             types.StringValue(a.Size),
			Region:           types.StringValue(a.Region),
			PHPVersion:       types.StringPointerValue(a.PHPVersion),
			PHPCLIVersion:    types.StringPointerValue(a.PHPCLIVersion),
			OpcacheStatus:    types.StringPointerValue(a.OpcacheStatus),
			DatabaseType:     types.StringPointerValue(a.DatabaseType),
			DBStatus:         types.StringPointerValue(a.DBStatus),
			RedisStatus:      types.StringPointerValue(a.RedisStatus),
			IPAddress:        types.StringPointerValue(a.IPAddress),
			PrivateIPAddress: types.StringPointerValue(a.PrivateIPAddress),
			Revoked:          types.BoolValue(a.Revoked),
			CreatedAt:        types.StringValue(a.CreatedAt),
			UpdatedAt:        types.StringValue(a.UpdatedAt),
			ConnectionStatus: types.StringValue(a.ConnectionStatus),
			Timezone:         types.StringValue(a.Timezone),
			LocalPublicKey:   types.StringPointerValue(a.LocalPublicKey),
			IsReady:          types.BoolValue(a.IsReady),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
