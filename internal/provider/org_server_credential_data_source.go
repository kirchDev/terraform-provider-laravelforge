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
	_ datasource.DataSource              = (*orgServerCredentialDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgServerCredentialDataSource)(nil)
)

// NewOrgServerCredentialDataSource returns a new
// laravelforge_org_server_credential data source.
//
// Reads a single org-scoped server credential by ID
// (`/api/orgs/{org}/server-credentials/{id}`). This is the read-side companion
// to the laravelforge_org_server_credential resource (which manages the
// team-share linkage). The credential object itself is read-only at the org
// scope. Reuses orgServerCredentialAttributes from the resource.
func NewOrgServerCredentialDataSource() datasource.DataSource {
	return &orgServerCredentialDataSource{}
}

type orgServerCredentialDataSource struct {
	client *client.Client
}

type orgServerCredentialDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ID            types.Int64  `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	InUse         types.Bool   `tfsdk:"in_use"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (d *orgServerCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_server_credential"
}

func (d *orgServerCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge org-scoped server credential by ID.",
		Attributes: map[string]schema.Attribute{
			"organization":   schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization that owns the credential.", Required: true},
			"id":             schema.Int64Attribute{MarkdownDescription: "Numeric ID of the credential.", Required: true},
			"name":           schema.StringAttribute{MarkdownDescription: "Credential name.", Computed: true},
			"cloud_provider": schema.StringAttribute{MarkdownDescription: "Provider the credential authenticates against, e.g. `Hetzner Cloud` (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
			"in_use":         schema.BoolAttribute{MarkdownDescription: "Whether the credential is currently in use by a server.", Computed: true},
			"created_at":     schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":     schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *orgServerCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgServerCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgServerCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single-credential reads are org-level (the resource's links.self).
	path := fmt.Sprintf("/api/orgs/%s/server-credentials/%d", data.Organization.ValueString(), data.ID.ValueInt64())
	var a orgServerCredentialAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server credential", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.CloudProvider = types.StringValue(a.Provider)
	data.InUse = types.BoolValue(a.InUse)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
