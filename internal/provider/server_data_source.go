package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// --- Data-source pattern exemplar. New data sources follow this shape. ---

var (
	_ datasource.DataSource              = (*serverDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverDataSource)(nil)
)

// NewServerDataSource returns a new laravelforge_server data source.
func NewServerDataSource() datasource.DataSource {
	return &serverDataSource{}
}

type serverDataSource struct {
	client *client.Client
}

// serverAttributes mirrors the JSON:API "attributes" of a server resource.
type serverAttributes struct {
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Type             string  `json:"type"`
	Provider         string  `json:"provider"`
	Region           string  `json:"region"`
	Size             string  `json:"size"`
	IPAddress        *string `json:"ip_address"`
	PrivateIPAddress *string `json:"private_ip_address"`
	PHPVersion       *string `json:"php_version"`
	IsReady          bool    `json:"is_ready"`
	CreatedAt        string  `json:"created_at"`
}

type serverDataSourceModel struct {
	Organization     types.String `tfsdk:"organization"`
	ID               types.Int64  `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	Type             types.String `tfsdk:"type"`
	CloudProvider    types.String `tfsdk:"cloud_provider"`
	Region           types.String `tfsdk:"region"`
	Size             types.String `tfsdk:"size"`
	IPAddress        types.String `tfsdk:"ip_address"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	PHPVersion       types.String `tfsdk:"php_version"`
	IsReady          types.Bool   `tfsdk:"is_ready"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (d *serverDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (d *serverDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge server by ID within an organization.",
		Attributes: map[string]schema.Attribute{
			"organization":       schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization that owns the server.", Required: true},
			"id":                 schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server.", Required: true},
			"name":               schema.StringAttribute{MarkdownDescription: "Server name.", Computed: true},
			"slug":               schema.StringAttribute{MarkdownDescription: "Server slug.", Computed: true},
			"type":               schema.StringAttribute{MarkdownDescription: "Server type (e.g. `app`, `web`, `database`).", Computed: true},
			"cloud_provider":     schema.StringAttribute{MarkdownDescription: "Underlying server provider (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
			"region":             schema.StringAttribute{MarkdownDescription: "Region the server runs in.", Computed: true},
			"size":               schema.StringAttribute{MarkdownDescription: "Server size / plan.", Computed: true},
			"ip_address":         schema.StringAttribute{MarkdownDescription: "Public IP address.", Computed: true},
			"private_ip_address": schema.StringAttribute{MarkdownDescription: "Private IP address.", Computed: true},
			"php_version":        schema.StringAttribute{MarkdownDescription: "Installed PHP version.", Computed: true},
			"is_ready":           schema.BoolAttribute{MarkdownDescription: "Whether the server has finished provisioning.", Computed: true},
			"created_at":         schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *serverDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serverDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d", data.Organization.ValueString(), data.ID.ValueInt64())
	var a serverAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Slug = types.StringValue(a.Slug)
	data.Type = types.StringValue(a.Type)
	data.CloudProvider = types.StringValue(a.Provider)
	data.Region = types.StringValue(a.Region)
	data.Size = types.StringValue(a.Size)
	data.IPAddress = types.StringPointerValue(a.IPAddress)
	data.PrivateIPAddress = types.StringPointerValue(a.PrivateIPAddress)
	data.PHPVersion = types.StringPointerValue(a.PHPVersion)
	data.IsReady = types.BoolValue(a.IsReady)
	data.CreatedAt = types.StringValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
