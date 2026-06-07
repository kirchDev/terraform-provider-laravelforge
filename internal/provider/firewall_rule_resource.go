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

// Firewall rules support create, read and delete only — the API exposes no
// PUT/PATCH (verified: the item route's "Supported methods" are GET, DELETE,
// and the OpenAPI spec has no update operation). Every settable attribute is
// therefore RequiresReplace and Update is a no-op that just re-reads.

var (
	_ resource.Resource                = (*firewallRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*firewallRuleResource)(nil)
	_ resource.ResourceWithImportState = (*firewallRuleResource)(nil)
)

// NewFirewallRuleResource returns a new laravelforge_firewall_rule resource.
func NewFirewallRuleResource() resource.Resource {
	return &firewallRuleResource{}
}

type firewallRuleResource struct {
	client *client.Client
}

type firewallRuleResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Port         types.String `tfsdk:"port"`
	Type         types.String `tfsdk:"type"`
	IPAddress    types.String `tfsdk:"ip_address"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *firewallRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rule"
}

func (r *firewallRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a firewall rule on a Laravel Forge server. Firewall rules cannot be updated in place; any change forces a replacement.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "Organization slug.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"server_id": schema.Int64Attribute{
				MarkdownDescription: "ID of the server to create the firewall rule on.",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"id": schema.Int64Attribute{
				MarkdownDescription: "Numeric ID of the firewall rule.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the firewall rule.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Rule type (`allow` or `deny`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"port": schema.StringAttribute{
				MarkdownDescription: "Port or port range for the firewall rule.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "IP address or subnet the rule applies to.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"status":     schema.StringAttribute{MarkdownDescription: "Provisioning status of the rule.", Computed: true},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last-update timestamp.", Computed: true},
		},
	}
}

func (r *firewallRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *firewallRuleResource) basePath(m *firewallRuleResourceModel) string {
	return fmt.Sprintf("/api/orgs/%s/servers/%d/firewall-rules", m.Organization.ValueString(), m.ServerID.ValueInt64())
}

func (r *firewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
		"type": plan.Type.ValueString(),
	}
	if !plan.Port.IsNull() && !plan.Port.IsUnknown() {
		body["port"] = plan.Port.ValueString()
	}
	if !plan.IPAddress.IsNull() && !plan.IPAddress.IsUnknown() {
		body["ip_address"] = plan.IPAddress.ValueString()
	}

	idStr, err := r.client.Write(ctx, "POST", r.basePath(&plan), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forge firewall rule", err.Error())
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected firewall rule ID", fmt.Sprintf("Forge returned a non-numeric firewall rule ID %q: %s", idStr, err))
		return
	}
	plan.ID = types.Int64Value(id)

	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read firewall rule after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readInto(ctx, &state); err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forge firewall rule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should never run: every settable attribute is RequiresReplace and the
// API has no update endpoint. It is implemented (the framework requires it) to
// just re-read and persist the plan.
func (r *firewallRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan firewallRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.readInto(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to read firewall rule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := fmt.Sprintf("%s/%d", r.basePath(&state), state.ID.ValueInt64())
	if err := r.client.Delete(ctx, itemPath); err != nil && !client.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Forge firewall rule", err.Error())
	}
}

// ImportState accepts "organization/server_id/firewall_rule_id".
func (r *firewallRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"organization/server_id/firewall_rule_id\".")
		return
	}
	serverID, err1 := strconv.ParseInt(parts[1], 10, 64)
	ruleID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		resp.Diagnostics.AddError("Invalid import ID", "server_id and firewall_rule_id must be numeric.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ruleID)...)
}

// readInto GETs the firewall rule identified by m and fills the
// computed/optional fields. Single-rule reads are server-scoped (per the
// resource's links.self).
func (r *firewallRuleResource) readInto(ctx context.Context, m *firewallRuleResourceModel) error {
	itemPath := fmt.Sprintf("%s/%d", r.basePath(m), m.ID.ValueInt64())
	var a firewallRuleAttributes
	if _, err := r.client.Get(ctx, itemPath, &a); err != nil {
		return err
	}
	m.Name = types.StringValue(a.Name)
	m.Port = types.StringPointerValue(a.Port)
	m.Type = types.StringValue(a.Type)
	m.IPAddress = types.StringPointerValue(a.IPAddress)
	m.Status = types.StringPointerValue(a.Status)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	return nil
}
