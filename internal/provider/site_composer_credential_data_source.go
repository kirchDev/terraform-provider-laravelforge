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
	_ datasource.DataSource              = (*siteComposerCredentialDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteComposerCredentialDataSource)(nil)
)

// NewSiteComposerCredentialDataSource returns a new laravelforge_site_composer_credential data source.
func NewSiteComposerCredentialDataSource() datasource.DataSource {
	return &siteComposerCredentialDataSource{}
}

type siteComposerCredentialDataSource struct {
	client *client.Client
}

type siteComposerCredentialDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Repository   types.String `tfsdk:"repository"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
}

func (d *siteComposerCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_composer_credential"
}

func (d *siteComposerCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge Composer authentication credential for a repository host on a site.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "Numeric ID of the server the site belongs to.", Required: true},
			"site_id":      schema.Int64Attribute{MarkdownDescription: "Numeric ID of the site the credential belongs to.", Required: true},
			"repository":   schema.StringAttribute{MarkdownDescription: "Repository host the credential authenticates against.", Required: true},
			"username":     schema.StringAttribute{MarkdownDescription: "Username for the Composer credential.", Computed: true},
			"password":     schema.StringAttribute{MarkdownDescription: "Password (or token) for the Composer credential.", Computed: true, Sensitive: true},
		},
	}
}

func (d *siteComposerCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteComposerCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteComposerCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/servers/%d/sites/%d/composer/credentials/%s",
		data.Organization.ValueString(), data.ServerID.ValueInt64(), data.SiteID.ValueInt64(), data.Repository.ValueString())
	var a siteComposerCredentialAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge composer credential", err.Error())
		return
	}

	data.Repository = types.StringValue(a.Repository)
	data.Username = types.StringValue(a.Username)
	data.Password = types.StringValue(a.Password)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
