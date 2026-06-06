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
	_ datasource.DataSource              = (*sslCertificateDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sslCertificateDataSource)(nil)
)

// NewSSLCertificateDataSource returns a new laravelforge_ssl_certificate data source.
func NewSSLCertificateDataSource() datasource.DataSource {
	return &sslCertificateDataSource{}
}

type sslCertificateDataSource struct {
	client *client.Client
}

type sslCertificateDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	DomainID     types.Int64  `tfsdk:"domain_id"`
	ID           types.Int64  `tfsdk:"id"`

	Domain        types.String `tfsdk:"domain"`
	Type          types.String `tfsdk:"type"`
	RequestStatus types.String `tfsdk:"request_status"`
	Status        types.String `tfsdk:"status"`
	Active        types.Bool   `tfsdk:"active"`
	Existing      types.Bool   `tfsdk:"existing"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func (d *sslCertificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssl_certificate"
}

func (d *sslCertificateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge SSL/TLS certificate by ID on a site domain.",
		Attributes: map[string]schema.Attribute{
			"organization":   schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":      schema.Int64Attribute{MarkdownDescription: "ID of the server the site belongs to.", Required: true},
			"site_id":        schema.Int64Attribute{MarkdownDescription: "ID of the site the domain belongs to.", Required: true},
			"domain_id":      schema.Int64Attribute{MarkdownDescription: "ID of the domain the certificate is installed on.", Required: true},
			"id":             schema.Int64Attribute{MarkdownDescription: "Numeric ID of the certificate.", Required: true},
			"domain":         schema.StringAttribute{MarkdownDescription: "Primary domain the certificate covers.", Computed: true},
			"type":           schema.StringAttribute{MarkdownDescription: "Certificate type.", Computed: true},
			"request_status": schema.StringAttribute{MarkdownDescription: "Certificate request status.", Computed: true},
			"status":         schema.StringAttribute{MarkdownDescription: "Certificate status.", Computed: true},
			"active":         schema.BoolAttribute{MarkdownDescription: "Whether this certificate is the active one for the domain.", Computed: true},
			"existing":       schema.BoolAttribute{MarkdownDescription: "Whether the certificate was uploaded (vs. issued by Forge).", Computed: true},
			"created_at":     schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
		},
	}
}

func (d *sslCertificateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sslCertificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data sslCertificateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Certificates are scoped per-domain in the new Forge SSL system.
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/domains/%d/certificates/%d",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(),
		data.DomainID.ValueInt64(), data.ID.ValueInt64())
	var a sslCertificateAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge SSL certificate", err.Error())
		return
	}

	data.Domain = types.StringValue(a.Domain)
	data.Type = types.StringValue(a.Type)
	data.RequestStatus = types.StringPointerValue(a.RequestStatus)
	data.Status = types.StringPointerValue(a.Status)
	data.Active = types.BoolValue(a.Active)
	data.Existing = types.BoolValue(a.Existing)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
