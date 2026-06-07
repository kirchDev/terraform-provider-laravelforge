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
	_ datasource.DataSource              = (*orgTeamInviteDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgTeamInviteDataSource)(nil)
)

// NewOrgTeamInviteDataSource returns a new laravelforge_org_team_invite data source.
func NewOrgTeamInviteDataSource() datasource.DataSource {
	return &orgTeamInviteDataSource{}
}

type orgTeamInviteDataSource struct {
	client *client.Client
}

type orgTeamInviteDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	TeamID       types.Int64  `tfsdk:"team_id"`
	ID           types.Int64  `tfsdk:"id"`
	Email        types.String `tfsdk:"email"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *orgTeamInviteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_invite"
}

func (d *orgTeamInviteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single pending Laravel Forge team invitation by ID.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"team_id":      schema.Int64Attribute{MarkdownDescription: "ID of the team.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the invitation.", Required: true},
			"email":        schema.StringAttribute{MarkdownDescription: "Email address of the invited member.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
		},
	}
}

func (d *orgTeamInviteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgTeamInviteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgTeamInviteDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/teams/%d/invites/%d", data.Organization.ValueString(), data.TeamID.ValueInt64(), data.ID.ValueInt64())
	var a orgTeamInviteAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge team invite", err.Error())
		return
	}

	data.Email = types.StringValue(a.Email)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
