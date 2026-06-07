package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSiteHorizonResource drives laravelforge_site_horizon, the exemplar of
// the integration-toggle singletons: it has no id, create/read/delete all hit
// the same path, create POSTs no body, and there is no update (every input is a
// RequiresReplace parent id). The mock runs in singleton mode (keyed by path).
func TestAccSiteHorizonResource(t *testing.T) {
	newMockForge(t, mockOpts{
		typeName:  "horizon",
		singleton: true,
		defaults: map[string]any{
			"enabled":           "true",
			"horizon_installed": true,
		},
	})

	const rn = "laravelforge_site_horizon.test"
	const cfg = `
resource "laravelforge_site_horizon" "test" {
  organization = "kirchdev"
  server_id    = 1
  site_id      = 2
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{ // create (enable) — POST with no body, then read the status
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "enabled", "true"),
					resource.TestCheckResourceAttr(rn, "horizon_installed", "true"),
				),
			},
			{ // import: organization/server_id/site_id. The resource has no `id`,
				// so point the verifier at a present identifier attribute.
				ResourceName:                         rn,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "site_id",
				ImportStateIdFunc:                    importIDFunc(rn, "organization", "server_id", "site_id"),
			},
		},
	})
}
