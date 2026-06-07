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
	_ datasource.DataSource              = (*orgServerCredentialLookupDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgServerCredentialLookupDataSource)(nil)
)

// NewOrgServerCredentialLookupDataSource returns a new
// laravelforge_org_server_credential_lookup data source.
//
// Server credentials are org-scoped provider credentials used to provision
// servers. This data source is the org-wide index: it GETs
// `/api/orgs/{org}/server-credentials` and returns every credential in the
// organization. It complements `laravelforge_credential` (single show by ID)
// and the team-scoped credential write resource — both read-only at the org
// parent scope (the index/show endpoints are GET-only).
func NewOrgServerCredentialLookupDataSource() datasource.DataSource {
	return &orgServerCredentialLookupDataSource{}
}

type orgServerCredentialLookupDataSource struct {
	client *client.Client
}

// orgServerCredentialLookupAttributes mirrors the JSON:API "attributes" of a
// ServerCredentialResource.
type orgServerCredentialLookupAttributes struct {
	Name      string  `json:"name"`
	Provider  string  `json:"provider"`
	InUse     bool    `json:"in_use"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

// orgServerCredentialLookupModel is one entry of the computed credentials list.
type orgServerCredentialLookupModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	InUse         types.Bool   `tfsdk:"in_use"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

type orgServerCredentialLookupDataSourceModel struct {
	Organization types.String                     `tfsdk:"organization"`
	Credentials  []orgServerCredentialLookupModel `tfsdk:"credentials"`
}

func (d *orgServerCredentialLookupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_server_credential_lookup"
}

func (d *orgServerCredentialLookupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Laravel Forge server credentials within an organization.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Slug of the Forge organization whose server credentials to list.", Required: true},
			"credentials": schema.ListNestedAttribute{
				MarkdownDescription: "Server credentials in the organization.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":             schema.StringAttribute{MarkdownDescription: "Numeric ID of the credential.", Computed: true},
						"name":           schema.StringAttribute{MarkdownDescription: "Credential name.", Computed: true},
						"cloud_provider": schema.StringAttribute{MarkdownDescription: "Provider the credential authenticates against, e.g. `Hetzner Cloud` (Forge API `provider`; renamed because `provider` is reserved in HCL).", Computed: true},
						"in_use":         schema.BoolAttribute{MarkdownDescription: "Whether the credential is currently in use by a server.", Computed: true},
						"created_at":     schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
						"updated_at":     schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *orgServerCredentialLookupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgServerCredentialLookupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgServerCredentialLookupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/server-credentials", data.Organization.ValueString())
	resources, err := d.client.List(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forge server credentials", err.Error())
		return
	}

	data.Credentials = make([]orgServerCredentialLookupModel, 0, len(resources))
	for _, r := range resources {
		var a orgServerCredentialLookupAttributes
		if len(r.Attributes) > 0 {
			if err := json.Unmarshal(r.Attributes, &a); err != nil {
				resp.Diagnostics.AddError("Unable to decode Forge server credential", err.Error())
				return
			}
		}
		data.Credentials = append(data.Credentials, orgServerCredentialLookupModel{
			ID:            types.StringValue(r.ID),
			Name:          types.StringValue(a.Name),
			CloudProvider: types.StringValue(a.Provider),
			InUse:         types.BoolValue(a.InUse),
			CreatedAt:     types.StringPointerValue(a.CreatedAt),
			UpdatedAt:     types.StringPointerValue(a.UpdatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
