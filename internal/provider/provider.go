package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/kirchDev/terraform-provider-laravelforge/internal/client"
)

// Ensure LaravelForgeProvider satisfies the provider.Provider interface.
var _ provider.Provider = (*LaravelForgeProvider)(nil)

// LaravelForgeProvider is the provider implementation.
type LaravelForgeProvider struct {
	// version is set to the release version on build, or "dev" for local builds.
	version string
}

// LaravelForgeProviderModel maps provider schema data to a Go type.
type LaravelForgeProviderModel struct {
	Token    types.String `tfsdk:"token"`
	Endpoint types.String `tfsdk:"endpoint"`
}

// New returns a function that instantiates the provider.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LaravelForgeProvider{version: version}
	}
}

func (p *LaravelForgeProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "laravelforge"
	resp.Version = p.version
}

func (p *LaravelForgeProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [Laravel Forge](https://forge.laravel.com) resources as code.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				MarkdownDescription: "Laravel Forge API token. May also be set via the `FORGE_TOKEN` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Forge API. Defaults to `https://forge.laravel.com`. May also be set via `FORGE_ENDPOINT`.",
				Optional:            true,
			},
		},
	}
}

func (p *LaravelForgeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config LaravelForgeProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Forge API token",
			"The provider cannot create the Forge API client because the token is unknown. "+
				"Set the value statically in the configuration or via the FORGE_TOKEN environment variable.",
		)
		return
	}

	// Env vars are the default; explicit config wins.
	token := os.Getenv("FORGE_TOKEN")
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	endpoint := os.Getenv("FORGE_ENDPOINT")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Forge API token",
			"Set the provider `token` argument or the FORGE_TOKEN environment variable.",
		)
		return
	}

	c := client.New(endpoint, token)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *LaravelForgeProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDaemonResource,
		NewDatabaseResource,
		NewDatabaseUserResource,
		NewFirewallRuleResource,
		NewMonitorResource,
		NewNginxTemplateResource,
		NewOrgRoleResource,
		NewOrgServerCredentialResource,
		NewOrgServerCredentialVpcResource,
		NewOrgStorageProviderResource,
		NewOrgTeamInviteResource,
		NewOrgTeamMemberResource,
		NewOrgTeamRecipeShareResource,
		NewOrgTeamServerShareResource,
		NewRecipeResource,
		NewRedirectRuleResource,
		NewSSLCertificateResource,
		NewScheduledJobResource,
		NewSecurityRuleResource,
		NewServerBackgroundProcessResource,
		NewServerBackupConfigurationResource,
		NewServerNetworkResource,
		NewServerPHPCLIVersionResource,
		NewServerPHPFpmConfigResource,
		NewServerPHPMaxExecutionTimeResource,
		NewServerPhpCliConfigResource,
		NewServerPhpMaxUploadSizeResource,
		NewServerPhpOpcacheResource,
		NewServerPhpPoolConfigResource,
		NewServerPhpSiteVersionResource,
		NewServerPhpVersionResource,
		NewServerResource,
		NewServerSSHKeyResource,
		NewSiteComposerCredentialResource,
		NewSiteDeployHookResource,
		NewSiteDeployKeyResource,
		NewSiteDeployScriptResource,
		NewSiteDomainNginxResource,
		NewSiteDomainResource,
		NewSiteEnvironmentResource,
		NewSiteHealthcheckResource,
		NewSiteHeartbeatResource,
		NewSiteHorizonResource,
		NewSiteInertiaResource,
		NewSiteLaravelMaintenanceResource,
		NewSiteLaravelSchedulerResource,
		NewSiteLoadBalancingResource,
		NewSiteNginxResource,
		NewSiteNpmCredentialResource,
		NewSiteOctaneResource,
		NewSitePulseResource,
		NewSitePushToDeployResource,
		NewSiteResource,
		NewSiteReverbResource,
		NewTeamResource,
		NewWebhookResource,
		NewWorkerResource,
	}
}

func (p *LaravelForgeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCredentialDataSource,
		NewCurrentUserDataSource,
		NewDaemonDataSource,
		NewDatabaseDataSource,
		NewDatabaseUserDataSource,
		NewFirewallRuleDataSource,
		NewForgeRecipeDataSource,
		NewMonitorDataSource,
		NewNginxTemplateDataSource,
		NewOrgRecipeRunDataSource,
		NewOrgRoleDataSource,
		NewOrgRolePermissionDataSource,
		NewOrgServerCredentialDataSource,
		NewOrgServerCredentialLookupDataSource,
		NewOrgServerCredentialVpcDataSource,
		NewOrgSiteLookupDataSource,
		NewOrgStorageProviderDataSource,
		NewOrgTeamInviteDataSource,
		NewOrgTeamMemberDataSource,
		NewOrgTeamRecipeShareDataSource,
		NewOrgTeamServerShareDataSource,
		NewOrganizationDataSource,
		NewPermissionDataSource,
		NewPredefinedRoleDataSource,
		NewProviderDataSource,
		NewProviderRegionDataSource,
		NewProviderRegionSizeDataSource,
		NewProviderSizeDataSource,
		NewRecipeDataSource,
		NewRedirectRuleDataSource,
		NewRegionDataSource,
		NewSSLCertificateDataSource,
		NewScheduledJobDataSource,
		NewSecurityRuleDataSource,
		NewServerArchiveDataSource,
		NewServerBackgroundProcessDataSource,
		NewServerBackupConfigurationDataSource,
		NewServerBackupInstanceDataSource,
		NewServerDataSource,
		NewServerEventDataSource,
		NewServerLogDataSource,
		NewServerNetworkDataSource,
		NewServerNetworkInfoDataSource,
		NewServerPHPCLIVersionDataSource,
		NewServerPHPFpmConfigDataSource,
		NewServerPHPMaxExecutionTimeDataSource,
		NewServerPhpCliConfigDataSource,
		NewServerPhpMaxUploadSizeDataSource,
		NewServerPhpOpcacheDataSource,
		NewServerPhpPoolConfigDataSource,
		NewServerPhpSiteVersionDataSource,
		NewServerPhpVersionDataSource,
		NewServerSSHKeyDataSource,
		NewSiteApplicationLogDataSource,
		NewSiteCommandDataSource,
		NewSiteComposerCredentialDataSource,
		NewSiteDataSource,
		NewSiteDeployHookDataSource,
		NewSiteDeployKeyDataSource,
		NewSiteDeployScriptDataSource,
		NewSiteDeploymentDataSource,
		NewSiteDeploymentStatusDataSource,
		NewSiteDomainConfigurationDataSource,
		NewSiteDomainDataSource,
		NewSiteDomainNginxDataSource,
		NewSiteEnvironmentDataSource,
		NewSiteHealthcheckDataSource,
		NewSiteHeartbeatDataSource,
		NewSiteHorizonDataSource,
		NewSiteInertiaDataSource,
		NewSiteLaravelMaintenanceDataSource,
		NewSiteLaravelSchedulerDataSource,
		NewSiteLoadBalancingDataSource,
		NewSiteNginxAccessLogDataSource,
		NewSiteNginxDataSource,
		NewSiteNginxErrorLogDataSource,
		NewSiteNpmCredentialDataSource,
		NewSiteOctaneDataSource,
		NewSitePulseDataSource,
		NewSiteReverbDataSource,
		NewTeamDataSource,
		NewWebhookDataSource,
		NewWorkerDataSource,
	}
}
