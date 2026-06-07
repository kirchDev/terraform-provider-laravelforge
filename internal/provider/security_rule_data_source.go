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
	_ datasource.DataSource              = (*securityRuleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*securityRuleDataSource)(nil)
)

// NewSecurityRuleDataSource returns a new laravelforge_security_rule data source.
func NewSecurityRuleDataSource() datasource.DataSource {
	return &securityRuleDataSource{}
}

type securityRuleDataSource struct {
	client *client.Client
}

// securityRuleAttributes mirrors the JSON:API "attributes" of a security rule
// (read shape). The object-valued "credentials" array is intentionally skipped
// in this pass (scalars only). Verified against the live API 2026-06-06.
type securityRuleAttributes struct {
	Name      string  `json:"name"`
	Path      *string `json:"path"`
	Status    *string `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type securityRuleDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Path         types.String `tfsdk:"path"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *securityRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_rule"
}

func (d *securityRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge security rule (HTTP basic-auth protection) on a site by ID.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the rule belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the security rule.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Security rule name.", Computed: true},
			"path":         schema.StringAttribute{MarkdownDescription: "Protected path (null protects all routes).", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (d *securityRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *securityRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data securityRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single security-rule reads are server+site-scoped (the resource's
	// links.self / live "Supported methods: GET" on the item path).
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/security-rules/%d",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.ID.ValueInt64())
	var a securityRuleAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge security rule", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Path = types.StringPointerValue(a.Path)
	data.Status = types.StringPointerValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
