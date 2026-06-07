package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDaemonResource drives laravelforge_daemon through create → update →
// import. The API group is "background-processes" (not "daemons"); processes is
// updated in place via PUT while command/user/directory are replace-only.
func TestAccDaemonResource(t *testing.T) {
	newMockForge(t, mockOpts{
		typeName: "background-processes",
		defaults: map[string]any{
			"status":     "running",
			"created_at": "2026-06-07T00:00:00Z",
		},
	})

	const rn = "laravelforge_daemon.test"
	cfg := func(processes int) string {
		return fmt.Sprintf(`
resource "laravelforge_daemon" "test" {
  organization = "kirchdev"
  server_id    = 1
  command      = "php artisan queue:work"
  user         = "forge"
  directory    = "/home/forge/app"
  processes    = %d
}
`, processes)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{ // create
				Config: cfg(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "command", "php artisan queue:work"),
					resource.TestCheckResourceAttr(rn, "user", "forge"),
					resource.TestCheckResourceAttr(rn, "processes", "1"),
					resource.TestCheckResourceAttrSet(rn, "id"),
				),
			},
			{ // update processes in place — daemon update is a PUT
				Config: cfg(3),
				Check:  resource.TestCheckResourceAttr(rn, "processes", "3"),
			},
			{ // import: organization/server_id/daemon_id
				ResourceName:      rn,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importIDFunc(rn, "organization", "server_id", "id"),
			},
		},
	})
}
