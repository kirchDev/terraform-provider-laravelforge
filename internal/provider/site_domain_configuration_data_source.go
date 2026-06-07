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
	_ datasource.DataSource              = (*siteDomainConfigurationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteDomainConfigurationDataSource)(nil)
)

// NewSiteDomainConfigurationDataSource returns a new
// laravelforge_site_domain_configuration data source.
func NewSiteDomainConfigurationDataSource() datasource.DataSource {
	return &siteDomainConfigurationDataSource{}
}

type siteDomainConfigurationDataSource struct {
	client *client.Client
}

// siteDomainConfigurationAttributes mirrors the JSON:API "attributes" of a
// single DomainRecordConfigurationResource (one DNS record to apply).
type siteDomainConfigurationAttributes struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
	TTL   *int64 `json:"ttl"`
}

// siteDomainConfigurationRecordModel is one entry of the computed list.
type siteDomainConfigurationRecordModel struct {
	Type  types.String `tfsdk:"type"`
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
	TTL   types.Int64  `tfsdk:"ttl"`
}

type siteDomainConfigurationDataSourceModel struct {
	Organization   types.String                         `tfsdk:"organization"`
	ServerID       types.Int64                          `tfsdk:"server_id"`
	SiteID         types.Int64                          `tfsdk:"site_id"`
	DomainRecordID types.Int64                          `tfsdk:"domain_record_id"`
	Configurations []siteDomainConfigurationRecordModel `tfsdk:"configurations"`
}

func (d *siteDomainConfigurationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_domain_configuration"
}

func (d *siteDomainConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the DNS configuration instructions for a Laravel Forge site domain record: the read-only list of DNS records the user must create to verify and route the domain.",
		Attributes: map[string]schema.Attribute{
			"organization":     schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":        schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site belongs to.", Required: true},
			"site_id":          schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the domain record belongs to.", Required: true},
			"domain_record_id": schema.Int64Attribute{MarkdownDescription: "Numeric ID of the domain record to fetch DNS configuration for.", Required: true},
			"configurations": schema.ListNestedAttribute{
				MarkdownDescription: "DNS records to apply for the domain record.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":  schema.StringAttribute{MarkdownDescription: "The type of DNS record (`A`, `CNAME`, or `TXT`).", Computed: true},
						"name":  schema.StringAttribute{MarkdownDescription: "The name of the DNS record.", Computed: true},
						"value": schema.StringAttribute{MarkdownDescription: "The value (IP address, CNAME target, TXT value) of the DNS record.", Computed: true},
						"ttl":   schema.Int64Attribute{MarkdownDescription: "The recommended TTL (Time to Live) for the DNS record, in seconds.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *siteDomainConfigurationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDomainConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteDomainConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The endpoint returns an array of DomainRecordConfigurationResource.
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/domains/%d/configurations",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.DomainRecordID.ValueInt64())
	resources, err := d.client.List(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge site domain configuration", err.Error())
		return
	}

	data.Configurations = make([]siteDomainConfigurationRecordModel, 0, len(resources))
	for _, r := range resources {
		var a siteDomainConfigurationAttributes
		if len(r.Attributes) > 0 {
			if err := json.Unmarshal(r.Attributes, &a); err != nil {
				resp.Diagnostics.AddError("Unable to decode Forge site domain configuration", err.Error())
				return
			}
		}
		data.Configurations = append(data.Configurations, siteDomainConfigurationRecordModel{
			Type:  types.StringValue(a.Type),
			Name:  types.StringValue(a.Name),
			Value: types.StringValue(a.Value),
			TTL:   types.Int64PointerValue(a.TTL),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
