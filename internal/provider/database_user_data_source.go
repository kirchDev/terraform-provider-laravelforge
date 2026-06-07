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
	_ datasource.DataSource              = (*databaseUserDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*databaseUserDataSource)(nil)
)

// NewDatabaseUserDataSource returns a new laravelforge_database_user data source.
func NewDatabaseUserDataSource() datasource.DataSource {
	return &databaseUserDataSource{}
}

type databaseUserDataSource struct {
	client *client.Client
}

type databaseUserDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (d *databaseUserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_user"
}

func (d *databaseUserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Laravel Forge database user by ID on a server.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{MarkdownDescription: "Organization slug.", Required: true},
			"server_id":    schema.Int64Attribute{MarkdownDescription: "ID of the server the database user belongs to.", Required: true},
			"id":           schema.Int64Attribute{MarkdownDescription: "Numeric ID of the database user.", Required: true},
			"name":         schema.StringAttribute{MarkdownDescription: "Name of the database user.", Computed: true},
			"status":       schema.StringAttribute{MarkdownDescription: "Provisioning status.", Computed: true},
			"created_at":   schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at":   schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (d *databaseUserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *databaseUserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data databaseUserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Single database-user reads are server-scoped (per the resource's
	// links.self), matching the create/update/delete collection scope.
	path := fmt.Sprintf("/api/orgs/%s/servers/%d/database/users/%d", data.Organization.ValueString(), data.ServerID.ValueInt64(), data.ID.ValueInt64())
	var a databaseUserAttributes
	if _, err := d.client.Get(ctx, path, &a); err != nil {
		resp.Diagnostics.AddError("Unable to read Forge database user", err.Error())
		return
	}

	data.Name = types.StringValue(a.Name)
	data.Status = types.StringValue(a.Status)
	data.CreatedAt = types.StringValue(a.CreatedAt)
	data.UpdatedAt = types.StringValue(a.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
