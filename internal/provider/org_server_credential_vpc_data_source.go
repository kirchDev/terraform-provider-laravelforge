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
	_ datasource.DataSource              = (*orgServerCredentialVpcDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgServerCredentialVpcDataSource)(nil)
)

// NewOrgServerCredentialVpcDataSource returns a new laravelforge_org_server_credential_vpc data source.
func NewOrgServerCredentialVpcDataSource() datasource.DataSource {
	return &orgServerCredentialVpcDataSource{}
}

type orgServerCredentialVpcDataSource struct {
	client *client.Client
}

type orgServerCredentialVpcDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	Credential   types.Int64  `tfsdk:"credential"`
	Region       types.String `tfsdk:"region"`
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	CIDRBlock    types.String `tfsdk:"cidr_block"`
}

func (d *orgServerCredentialVpcDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_server_credential_vpc"
}

func (d *orgServerCredentialVpcDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single VPC (private network) at a server provider by ID, via a Forge organization's server credential and region.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"credential":   schema.Int64Attribute{MarkdownDescription: "ID of the server credential used to reach the provider.", Required: true},
			"region":       schema.StringAttribute{MarkdownDescription: "Provider region the VPC is in.", Required: true},
			"id":           schema.StringAttribute{MarkdownDescription: "ID of the VPC at the provider.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Name of the VPC.", Computed: true},
			"cidr_block":   schema.StringAttribute{MarkdownDescription: "CIDR block of the VPC.", Computed: true},
		},
	}
}

func (d *orgServerCredentialVpcDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgServerCredentialVpcDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgServerCredentialVpcDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/server-credentials/%d/regions/%s/vpcs/%s",
		data.Organization.ValueString(), data.Credential.ValueInt64(), data.Region.ValueString(), data.ID.ValueString())
	var a orgServerCredentialVpcAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge VPC", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.CIDRBlock = types.StringValue(a.CIDRBlock)
	data.Region = types.StringValue(a.Region)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
