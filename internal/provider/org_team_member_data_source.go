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
	_ datasource.DataSource              = (*orgTeamMemberDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgTeamMemberDataSource)(nil)
)

// NewOrgTeamMemberDataSource returns a new laravelforge_org_team_member data source.
func NewOrgTeamMemberDataSource() datasource.DataSource {
	return &orgTeamMemberDataSource{}
}

type orgTeamMemberDataSource struct {
	client *client.Client
}

type orgTeamMemberDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	TeamID       types.Int64  `tfsdk:"team_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Email        types.String `tfsdk:"email"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *orgTeamMemberDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_team_member"
}

func (d *orgTeamMemberDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single member of a Laravel Forge team by user ID.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"team_id":      schema.Int64Attribute{MarkdownDescription: "ID of the team the membership belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the user whose membership is fetched.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Member's name.", Computed: true},
			"email":        schema.StringAttribute{MarkdownDescription: "Member's email address.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Membership creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Membership update timestamp.", Computed: true},
		},
	}
}

func (d *orgTeamMemberDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgTeamMemberDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgTeamMemberDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/orgs/%s/teams/%d/members/%d", data.Organization.ValueString(), data.TeamID.ValueInt64(), data.ID.ValueInt64())
	var a orgTeamMemberAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge team member", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Email = types.StringValue(a.Email)
	data.CreatedAt = types.StringPointerValue(a.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
