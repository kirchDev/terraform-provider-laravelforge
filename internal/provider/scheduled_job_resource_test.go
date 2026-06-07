package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccScheduledJobResource drives laravelforge_scheduled_job through create →
// import. Forge has no update endpoint, so every input is replace-only.
//
// The mock Capitalizes the frequency it echoes back (the real API returns
// "Custom" for a "custom" request). The resource deliberately does NOT refresh
// command/user/frequency/name from the read, so state keeps the lowercase
// config value and the post-apply refresh stays drift-free — this test fails if
// that guard regresses. Those same fields aren't returned by the read, so they
// can't round-trip through import and are verify-ignored.
func TestAccScheduledJobResource(t *testing.T) {
	newMockForge(t, mockOpts{
		typeName: "scheduled-jobs",
		defaults: map[string]any{
			"status":        "installed",
			"next_run_time": "2026-06-08T00:00:00Z",
			"created_at":    "2026-06-07T00:00:00Z",
			"updated_at":    "2026-06-07T00:00:00Z",
		},
		onRead: func(attrs map[string]any) {
			// Emulate the API echoing a Capitalized frequency ("custom" → "Custom").
			if f, ok := attrs["frequency"].(string); ok && f != "" {
				attrs["frequency"] = strings.ToUpper(f[:1]) + f[1:]
			}
		},
	})

	const rn = "laravelforge_scheduled_job.test"
	const cfg = `
resource "laravelforge_scheduled_job" "test" {
  organization = "kirchdev"
  server_id    = 1
  command      = "php artisan schedule:run"
  user         = "forge"
  frequency    = "custom"
  cron         = "0 0 * * *"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{ // create — frequency must stay lowercase despite the API capitalizing it
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "command", "php artisan schedule:run"),
					resource.TestCheckResourceAttr(rn, "frequency", "custom"),
					resource.TestCheckResourceAttr(rn, "cron", "0 0 * * *"),
					resource.TestCheckResourceAttr(rn, "status", "installed"),
					resource.TestCheckResourceAttrSet(rn, "id"),
				),
			},
			{ // import: organization/server_id/scheduled_job_id. command/user/
				// frequency aren't returned by the read, so they can't round-trip.
				ResourceName:            rn,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"command", "user", "frequency"},
				ImportStateIdFunc:       importIDFunc(rn, "organization", "server_id", "id"),
			},
		},
	})
}
