# TEMP_AI.md — Laravel Forge OpenTofu provider (pickup runbook)

> Scratch/handoff doc for AI sessions and future-me. Delete once the build is
> real and the facts have moved into `CLAUDE.md` + `README.md`. Mirrors the
> style of the infra repo's `ToDo/linear.md`.

## What this repo is (and why we're building it)

A **first-party OpenTofu provider for Laravel Forge**, owned by kirchDev. We
manage our infra estate as code (GitHub, Cloudflare, Better Stack, Linear);
Forge is the next target and **no usable provider exists**:

- `madewithlove/laravelforge` (v0.1.1) — **non-functional stub**. `provider.go`
  registers ZERO resources/data sources (`return []func()...{}`). The
  `*_resource_gen.go` files are auto-generated schema scaffolding (from Forge's
  OpenAPI via terraform-plugin-framework-generator) with **no CRUD and not
  wired in**. Docs are generated artifacts only. → unusable.
- `tonning/laravelforge` (v1.0.6, SDKv2) — the only one that actually works
  (server/site/key/sslcertificate/scheduledjob/daemon/redirectrule + 2 data
  sources, real CRUD), but **thin** vs the full API and **~18 mo unmaintained**
  (last commit 2024-12-27). Not a base to build on (SDKv2 = legacy).
- The Forge **REST API is complete** (`https://forge.laravel.com/api`, bearer
  token) → a much better provider is clearly buildable. We write our own.

Full evaluation lives in the infra repo memory `tf-service-expansion-eval`.

## Locked facts (don't relitigate)

| Thing | Value |
| --- | --- |
| GitHub repo | `kirchDev/terraform-provider-laravelforge` (org, public) |
| Provider type (HCL) | `laravelforge` → `provider "laravelforge" {}` |
| Registry address | `kirchdev/laravelforge` (OpenTofu registry) |
| SDK | **terraform-plugin-framework** (NOT SDKv2) |
| Auth | provider arg `token` + `FORGE_TOKEN` env; in infra later from Bitwarden SM |
| Versioning | pre-1.0; infra repo pins **exactly** (like ucodecov/linear) |

The repo name format `NAMESPACE/terraform-provider-NAME` is **mandatory** for
the OpenTofu/Terraform registry — not a style choice.

## Status

| Piece | State |
| --- | --- |
| Repo created from `TitusKirch/scaffold`, rebranded to the provider | ✅ README/CLAUDE/package.json/CONTRIBUTING/SECURITY/issue+PR templates/release-please all retargeted |
| Go module / provider skeleton | ✅ `go.mod` + `main.go` + provider (`token`/`endpoint` auth) + `internal/client` + `.gitignore`/`GNUmakefile`/`terraform-registry-manifest.json`. **Builds** (`go build .`) and **loads in OpenTofu** (`tofu validate` via `dev_overrides` green). Go ≥ 1.25 required (framework v1.19). |
| First **data source** `laravelforge_server` | ✅ wired + **verified end-to-end against the live API** (`tofu plan` read a real server). Two issues caught + fixed: `provider` is a reserved HCL attr name → `cloud_provider`; the API is strict JSON:API (fields under `attributes`, resource `id` is a string while `attributes.id` is numeric) so the client decodes a `serverDocument`/`serverResource`/`serverAttributes` shape, not a flat object. Org slug `kirchdev`. |
| **All manageable resources + data sources** | ✅ built via multi-agent workflow (`.workflow-build-all.js`, gitignored). **17 resources + 19 data sources**: server, site, daemon, scheduled_job, firewall_rule, database, database_user, nginx_template, monitor, server_ssh_key, worker, redirect_rule, security_rule, webhook, ssl_certificate, recipe, team (+ read-only `credential`/`region` data sources). `go build`/`vet` clean, `tofu validate` green across all schemas, reads verified live. **Writes NOT acceptance-tested** (read-only token). Per-entity gotchas captured in code/commit + the workflow result; key ones: daemon's API group is `background-processes` (not `daemons`); scheduled_job has no update endpoint (all-RequiresReplace) and request `frequency` is lowercase while the response capitalizes it; firewall/redirect/security rules needed scope the read token lacked (routes confirmed via 405/OpenAPI). |
| **Full API coverage (wave 2)** | ✅ second workflow (`.workflow-build-rest.js`) planned from the OpenAPI spec (`.forge-openapi.json`, gitignored) + fanned out. Now **~57 resources + ~83 data sources** — every non-action entity: server PHP/network/logs, site env/deployments/commands/workers/SSL/load-balancing/webhooks + 7 integration toggles, db backups, roles & permissions, storage providers, server credentials, VPCs, providers/regions/sizes + org/user data sources. `go build`/`vet`/`gofmt`/`tofu validate` all green (verified independently); new data sources read live (organization, current_user confirmed). Type names are clean snake_case; no duplicates with wave 1. |
| Remaining for resources | ⛔ write acceptance-tests (write-scoped token); nested object/array attributes (scalars only so far); pure actions intentionally excluded; goreleaser+GPG+CI; tfplugindocs |
| OpenAPI → schema/client codegen | ⛔ not set up (spec not openly downloadable; revisit) |
| goreleaser + GPG signing | ⛔ |
| OpenTofu registry submission | ⛔ |

> Done in mismatch checklist below: #1 (Go toolchain/skeleton — partial: golangci-lint config still to add), #2 (`.gitignore`), #6 (placeholders/README/CLAUDE). Still open: #3 (release-please still `node`), #4 (CI Go job), #5 (goreleaser). Go 1.26.4 was installed to `/usr/local/go` in the dev sandbox (not in the repo).

## ⚠️ Template mismatch — scaffold is Node, this is Go

The repo shipped the kirchDev `scaffold` template: **pnpm + oxlint + oxfmt +
husky + commitlint + release-please**, tuned for TS/Bun tools (envprism,
forgemap). A TF provider is a **Go** project. Conversion to-do:

1. **Add the Go toolchain.** `go mod init github.com/kirchDev/terraform-provider-laravelforge`,
   `main.go` (`providerserver.Serve`), `internal/provider/`. Add `golangci-lint`
   + `gofmt`/`gofumpt`. Decide whether the Node lint layer (oxlint/oxfmt over
   JSON/YAML/MD) stays for meta files or gets dropped — leaning **keep** for
   config/docs, add Go linters alongside.
2. **`.gitignore`** — currently Node-only. Add Go/TF artifacts: built binary
   `terraform-provider-laravelforge`, `/dist`, `.terraform/`, `*.tfstate*`,
   `*.tfrc`, coverage files.
3. **release-please** — currently `release-type: node`, `package-name: scaffold`,
   manifest pinned `0.1.1`. Fix: switch to `release-type: simple` (or `go`),
   rename `package-name` → `terraform-provider-laravelforge`, **reset manifest to
   `0.0.0`** (scaffold reset steps in its README). It still owns the changelog +
   the version tag; the tag then triggers goreleaser (next item).
4. **CI (`ci.yml`)** — currently `pnpm lint`/`format`. Add a Go job:
   `go build`, `go vet`, `golangci-lint`, `go test ./...` (incl. acceptance tests
   behind `TF_ACC`, which need a throwaway Forge token — gate so they don't run
   on forks/PRs without the secret).
5. **Add `.goreleaser.yml`** + a `release.yml` workflow that fires **on the tag**
   release-please pushes: builds the standard provider artifacts
   (`terraform-provider-laravelforge_<ver>_<os>_<arch>.zip`, `_SHA256SUMS`,
   `_SHA256SUMS.sig`, `manifest.json`) and **GPG-signs** them. Copy HashiCorp's
   `terraform-provider-scaffolding-framework` release workflow as the base.
6. **Replace scaffold placeholders** (`grep -rn "TitusKirch/scaffold" .`),
   regenerate `CLAUDE.md` (current one is scaffold-specific), rewrite README in
   house style for a provider.
7. Note: `dev-pr.yml` (dev→main rollup) is inherited — fine to keep; matches the
   kirchDev flow.

## Build plan

**Leverage Forge's OpenAPI spec** — this is the whole point (it's what
madewithlove half-did and abandoned):

1. Generate a **Go API client** from the OpenAPI spec (`oapi-codegen` or similar).
2. Generate **TF Framework schemas** from the same spec
   (`terraform-plugin-codegen-openapi` → provider code spec →
   `terraform-plugin-codegen-framework`). This skips hand-typing ~25 resources.
3. Write the **CRUD glue** per resource (the gap the generators leave) and
   **register** each in `provider.go` `Resources()`/`DataSources()` — the exact
   step madewithlove never did.

**Vertical slice first, not breadth.** Ship `forge_server` + `forge_site`
end-to-end (real CRUD + one acceptance test against a disposable Forge account)
before adding more. Then expand resource-by-resource. Target surface (from the
Forge API), roughly in priority order:

- `forge_server`, `forge_site` (slice 1)
- `forge_database`, `forge_database_user`, `forge_ssl_certificate`
- `forge_daemon`, `forge_scheduled_job`, `forge_firewall_rule`,
  `forge_security_rule`, `forge_redirect_rule`
- `forge_ssh_key`, `forge_recipe`, `forge_nginx_template`, `forge_php_version`,
  `forge_deployment`, env vars, monitors, teams/orgs, integrations
- data sources for the read-heavy ones (`forge_server`, `forge_site`, …)

`forge_server` needs `credential_id` + `platform` (`ocean2`/`aws`/`hetzner`/
`custom`/…). For our own Hetzner story we'll likely lean on `platform=custom`
(server provisioned by `hetznercloud/hcloud` in tofu, then handed to Forge) —
see the memory note; that decision lives in the infra repo, not here.

## Release & registry

1. goreleaser builds + GPG-signs on tag (see mismatch #5).
2. Submit to the OpenTofu registry via the **issue form** in `opentofu/registry`
   (automation validates + opens the PR).
3. Submit the **GPG public key** the same way — registry verifies the submitter
   is a **public member** of the `kirchDev` org, so Titus's org membership must
   be set public first.
4. In the infra repo, add the provider with an **exact** version pin and onboard
   this repo into `tofu/data/github-orgs/<owner>.yml`.

## References

- HashiCorp scaffolding: `terraform-provider-scaffolding-framework`
- Plugin Framework docs + `terraform-plugin-codegen-openapi` / `-framework`
- Forge API: https://forge.laravel.com/docs/api-reference/introduction
- OpenTofu publishing: https://github.com/opentofu/registry/blob/main/PROCEDURES.md
- Working reference (SDKv2, don't copy structure): `tonning/laravelforge`
