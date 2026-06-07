package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccServerResource drives laravelforge_server through create → update →
// import. Create/update/delete and the single-resource read are all org-level;
// name is updated in place via PUT while type/cloud_provider are replace-only.
func TestAccServerResource(t *testing.T) {
	newMockForge(t, mockOpts{
		typeName: "servers",
		defaults: map[string]any{
			"slug":       "web-1",
			"region":     "fra1",
			"size":       "cx21",
			"is_ready":   true,
			"created_at": "2026-06-07T00:00:00Z",
		},
	})

	const rn = "laravelforge_server.test"
	cfg := func(name string) string {
		return fmt.Sprintf(`
resource "laravelforge_server" "test" {
  organization   = "kirchdev"
  name           = %q
  type           = "app"
  cloud_provider = "hetzner"
}
`, name)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{ // create
				Config: cfg("web-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "name", "web-1"),
					resource.TestCheckResourceAttr(rn, "type", "app"),
					resource.TestCheckResourceAttr(rn, "cloud_provider", "hetzner"),
					resource.TestCheckResourceAttr(rn, "region", "fra1"),
					resource.TestCheckResourceAttrSet(rn, "id"),
				),
			},
			{ // update name in place — server update is a PUT
				Config: cfg("web-2"),
				Check:  resource.TestCheckResourceAttr(rn, "name", "web-2"),
			},
			{ // import: organization/id
				ResourceName:      rn,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importIDFunc(rn, "organization", "id"),
			},
		},
	})
}
