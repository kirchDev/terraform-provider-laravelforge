package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSiteResource drives laravelforge_site through create → update → import
// against the mockForge. site is the exemplar: create/update/delete are
// server-scoped, the single-resource read is org-level (per links.self), and
// update is PUT. Runs under TF_ACC with a TF binary; no token, no real Forge.
func TestAccSiteResource(t *testing.T) {
	newMockForge(t, mockOpts{
		typeName: "sites",
		defaults: map[string]any{
			"status":                    "installed",
			"url":                       "http://example.test",
			"created_at":                "2026-06-07T00:00:00Z",
			"zero_downtime_deployments": false,
		},
	})

	const rn = "laravelforge_site.test"
	cfg := func(php string) string {
		return fmt.Sprintf(`
resource "laravelforge_site" "test" {
  organization = "kirchdev"
  server_id    = 1
  type         = "php"
  name         = "example.test"
  php_version  = %q
}
`, php)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{ // create
				Config: cfg("php83"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "name", "example.test"),
					resource.TestCheckResourceAttr(rn, "php_version", "php83"),
					resource.TestCheckResourceAttr(rn, "status", "installed"),
					resource.TestCheckResourceAttrSet(rn, "id"),
				),
			},
			{ // update php_version in place — site update is a PUT
				Config: cfg("php84"),
				Check:  resource.TestCheckResourceAttr(rn, "php_version", "php84"),
			},
			{ // import: org/server_id/site_id. `type` isn't returned by the read
				// path, so it can't round-trip and is verify-ignored.
				ResourceName:            rn,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"type"},
				ImportStateIdFunc:       importIDFunc(rn, "organization", "server_id", "id"),
			},
		},
	})
}
