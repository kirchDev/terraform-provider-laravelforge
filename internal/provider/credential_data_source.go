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
	_ datasource.DataSource              = (*credentialDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*credentialDataSource)(nil)
)

// NewCredentialDataSource returns a new laravelforge_credential data source.
//
// Credentials (Forge "server credentials") are org-scoped provider credentials
// used to provision servers. At the org parent scope they are read-only: the
// list (`/api/orgs/{org}/server-credentials`) and single-read
// (`/api/orgs/{org}/server-credentials/{id}`) endpoints are GET-only —
// create/delete live under the team scope, so no resource is implemented here.
func NewCredentialDataSource() datasource.DataSource {
	return &credentialDataSource{}
}

type credentialDataSource struct {
	client *client.Client
}

// credentialAttributes mirrors the JSON:API "attributes" of a server credential.
type credentialAttributes struct {
	Name      string  `json:"name"`
	Provider  string  `json:"provider"`
	InUse     bool    `json:"in_use"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type credentialDataSourceModel struct {
	Organization  types.String `tfsdk:"organization"`
	ID            types.Int64  `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	InUse         types.Bool   `tfsdk:"in_use"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (d *credentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (d *credentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge server credential by ID within an organization.",
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

func (d *credentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *credentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data credentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single-credential reads are org-level (the resource's links.self).
	path := fmt.Sprintf("/api/orgs/%s/server-credentials/%d", data.Organization.ValueString(), data.ID.ValueInt64())
	var a credentialAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge credential", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.CloudProvider = types.StringValue(a.Provider)
	data.InUse = types.BoolValue(a.InUse)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
