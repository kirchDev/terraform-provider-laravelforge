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
	_ datasource.DataSource              = (*siteNpmCredentialDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteNpmCredentialDataSource)(nil)
)

// NewSiteNpmCredentialDataSource returns a new laravelforge_site_npm_credential
// data source.
func NewSiteNpmCredentialDataSource() datasource.DataSource {
	return &siteNpmCredentialDataSource{}
}

type siteNpmCredentialDataSource struct {
	client *client.Client
}

type siteNpmCredentialDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Registry     types.String `tfsdk:"registry"`
	Token        types.String `tfsdk:"token"`
}

func (d *siteNpmCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_npm_credential"
}

func (d *siteNpmCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single NPM registry credential for a Laravel Forge site by registry.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server hosting the site.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "ID of the site the credential belongs to.", Required: true},
			"registry":     schema.StringAttribute{MarkdownDescription: "The NPM registry URL the credential authenticates against.", Required: true},
			"token":        schema.StringAttribute{MarkdownDescription: "The authentication token for the registry.", Computed: true, Sensitive: true},
		},
	}
}

func (d *siteNpmCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteNpmCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteNpmCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/npm/credentials/%s",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.Registry.ValueString())
	var a siteNpmCredentialAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge NPM credential", err.Error())
		return
	}

	if a.Registry != "" {
		data.Registry = types.StringValue(a.Registry)
	}
	data.Token = types.StringValue(a.Token)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
