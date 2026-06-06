package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ datasource.DataSource              = (*firewallRuleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*firewallRuleDataSource)(nil)
)

// NewFirewallRuleDataSource returns a new laravelforge_firewall_rule data source.
func NewFirewallRuleDataSource() datasource.DataSource {
	return &firewallRuleDataSource{}
}

type firewallRuleDataSource struct {
	client *client.Client
}

// firewallRuleAttributes mirrors the JSON:API "attributes" of a firewall rule.
// Note: the API exposes `port` as a (nullable) string, not an integer.
type firewallRuleAttributes struct {
	Name      string  `json:"name"`
	Port      *string `json:"port"`
	Type      string  `json:"type"`
	IPAddress *string `json:"ip_address"`
	Status    *string `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type firewallRuleDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Port         types.String `tfsdk:"port"`
	Type         types.String `tfsdk:"type"`
	IPAddress    types.String `tfsdk:"ip_address"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *firewallRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rule"
}

func (d *firewallRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge firewall rule by ID on a server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the firewall rule belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the firewall rule.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Name of the firewall rule.", Computed: true},
			"port":         schema.StringAttribute{MarkdownDescription: "Port or port range for the firewall rule.", Computed: true},
			"type":         schema.StringAttribute{MarkdownDescription: "Rule type (`allow` or `deny`).", Computed: true},
			"ip_address":   schema.StringAttribute{MarkdownDescription: "IP address or subnet the rule applies to.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Provisioning status of the rule.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (d *firewallRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *firewallRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data firewallRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Firewall-rule reads are server-scoped (the resource's links.self), unlike
	// sites whose single read is org-level.
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/firewall-rules/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a firewallRuleAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge firewall rule", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Port = types.StringPointerValue(a.Port)
	data.Type = types.StringValue(a.Type)
	data.IPAddress = types.StringPointerValue(a.IPAddress)
	data.Status = types.StringPointerValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
