package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDatabaseResource drives laravelforge_database through create → import.
// Forge has no update endpoint for a database schema, so every input is
// replace-only and there is no in-place update step. user/password are
// write-only create inputs the API never returns, so they cannot round-trip
// through import and are verify-ignored.
func TestAccDatabaseResource(t *testing.T) {
	newMockForge(t, mockOpts{
		typeName: "database-schemas",
		defaults: map[string]any{
			"status":     "installed",
			"created_at": "2026-06-07T00:00:00Z",
			"updated_at": "2026-06-07T00:00:00Z",
		},
	})

	const rn = "laravelforge_database.test"
	const cfg = `
resource "laravelforge_database" "test" {
  organization = "kirchdev"
  server_id    = 1
  name         = "app_production"
  user         = "app_user"
  password     = "s3cret-pw"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{ // create (with a database user provisioned alongside)
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "name", "app_production"),
					resource.TestCheckResourceAttr(rn, "status", "installed"),
					resource.TestCheckResourceAttrSet(rn, "id"),
				),
			},
			{ // import: organization/server_id/database_id
				ResourceName:            rn,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"user", "password"},
				ImportStateIdFunc:       importIDFunc(rn, "organization", "server_id", "id"),
			},
		},
	})
}
