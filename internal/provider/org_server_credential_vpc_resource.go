package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

var (
	_ resource.Resource                = (*orgServerCredentialVpcResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgServerCredentialVpcResource)(nil)
	_ resource.ResourceWithImportState = (*orgServerCredentialVpcResource)(nil)
)

// NewOrgServerCredentialVpcResource returns a new laravelforge_org_server_credential_vpc resource.
func NewOrgServerCredentialVpcResource() resource.Resource {
	return &orgServerCredentialVpcResource{}
}

type orgServerCredentialVpcResource struct {
	client *client.Client
}

// orgServerCredentialVpcAttributes mirrors the JSON:API "attributes" of a VPC
// resource (read shape). The nested "subnets" array is intentionally omitted —
// only scalar attributes are mapped this pass.
type orgServerCredentialVpcAttributes struct {
	Name      string `json:"name"`
	CIDRBlock string `json:"cidrBlock"`
	Region    string `json:"region"`
}

type orgServerCredentialVpcResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	Credential   types.Int64  `tfsdk:"credential"`
	Region       types.String `tfsdk:"region"`
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	CIDRBlock    types.String `tfsdk:"cidr_block"`
}

func (r *orgServerCredentialVpcResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_server_credential_vpc"
}

func (r *orgServerCredentialVpcResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a VPC (private network) created at a server provider via a Forge organization's server credential and region. " +
			"The Forge API exposes only create and read for VPCs (no update, no delete), so this resource is create-and-read only: " +
			"every attribute is create-time, and destroying the resource removes it from state without deleting the VPC at the provider.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"credential": schema.Int64Attribute{
				MarkdownDescription: "ID of the server credential used to reach the provider.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Provider region the VPC is created in.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the VPC at the provider.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the VPC. Create-only.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cidr_block": schema.StringAttribute{
				MarkdownDescription: "CIDR block of the VPC.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *orgServerCredentialVpcResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = c
}

// collectionPath is the index/create path (no vpcId).
func (r *orgServerCredentialVpcResource) collectionPath(m *orgServerCredentialVpcResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/server-credentials/%d/regions/%s/vpcs",
		m.Organization.ValueString(), m.Credential.ValueInt64(), m.Region.ValueString())
}

// itemPath is the show path for a single VPC.
func (r *orgServerCredentialVpcResource) itemPath(m *orgServerCredentialVpcResourceModel) string {
	return fmt.Sprintf("%s/%s", r.collectionPath(m), m.ID.ValueString())
}

func (r *orgServerCredentialVpcResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgServerCredentialVpcResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"name": plan.Name.ValueString()}

	id, err := r.client.Write(ctx, "POST", r.collectionPath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge VPC", err.Error())
		return
	}
	plan.ID = types.StringValue(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read VPC after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgServerCredentialVpcResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgServerCredentialVpcResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge VPC", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable: every attribute is RequiresReplace and the API has no
// update endpoint. It exists only to satisfy the resource.Resource interface.
func (r *orgServerCredentialVpcResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan orgServerCredentialVpcResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a state-only removal: the Forge API exposes no DELETE for VPCs, so
// the VPC is left in place at the provider and merely dropped from state.
func (r *orgServerCredentialVpcResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts "organization/credential/region/vpc_id".
func (r *orgServerCredentialVpcResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/credential/region/vpc_id\".")
		return
	}
	credential, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "credential must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("credential"), credential)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("region"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[3])...)
}

// readInto GETs the VPC identified by m and fills the computed fields.
func (r *orgServerCredentialVpcResource) readInto(ctx context.Context, m *orgServerCredentialVpcResourceModel) error {
	var a orgServerCredentialVpcAttributes
	if _, err := r.client.Get(ctx, r.itemPath(m), &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.CIDRBlock = types.StringValue(a.CIDRBlock)
	m.Region = types.StringValue(a.Region)
	return nil
}
