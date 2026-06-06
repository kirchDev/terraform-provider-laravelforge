# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A first-party **OpenTofu / Terraform provider for [Laravel Forge](https://forge.laravel.com)**, owned by kirchDev. It manages Forge resources (servers, sites, databases, SSL certificates, daemons, scheduled jobs, …) as code so the Forge estate can live in the same IaC workflow as the rest of the kirchDev infrastructure.

- **Provider type (HCL):** `laravelforge` → `provider "laravelforge" {}`
- **OpenTofu registry address:** `kirchdev/laravelforge`
- **Go module:** `github.com/kirchDev/terraform-provider-laravelforge`
- **SDK:** `terraform-plugin-framework` (NOT the legacy SDKv2)

The repo name format `NAMESPACE/terraform-provider-NAME` is **mandatory** for the OpenTofu/Terraform registry — it's not a style choice.

> [!IMPORTANT]
> **Read `TEMP_AI.md` first.** It is the live build runbook: current status, the scaffold→Go conversion checklist, the OpenAPI-codegen plan, the resource roadmap, and the release/registry steps. Delete it once the build is real and its facts have moved into this file + the README.

## Current state (early)

Two layers coexist:

- **Node meta layer** (kept from the `scaffold` template): pnpm + oxlint + oxfmt + husky + commitlint + release-please + CI/CodeQL/Dependabot + issue/PR templates. Lints config/docs (JSON/YAML/MD).
- **Go provider** (added on top): `go.mod` (`terraform-plugin-framework`), `main.go`, `internal/client/` (generic JSON:API client: `Get`/`List`/`Write`/`Delete`/`NotFound`), and `internal/provider/` with **~57 resources + ~83 data sources** covering essentially the full manageable surface of the new org-scoped API (servers + PHP/network/logs, sites + env/deployments/commands/workers/rules/SSL/load-balancing/webhooks/integration-toggles, databases + users + backups, daemons, scheduled jobs, firewall rules, nginx templates, monitors, SSH keys, recipes, teams, roles & permissions, storage providers, server credentials, VPCs; read-only data sources for providers/regions/sizes, organizations, current user, etc.). Pure action endpoints (reboot/restart/deploy-trigger) are intentionally excluded. Everything **builds** (`go build ./...`), **vets**, and **loads** (`tofu validate` green across all ~140 schemas); **reads** are verified against the live API.

Still pending (see `TEMP_AI.md`): **write paths are not acceptance-tested** (read-only discovery token — Create/Update/Delete need a write-scoped token); list data sources; OpenAPI-driven codegen; tfplugindocs; the goreleaser + GPG release workflow; per-entity field fidelity (object/array attributes were skipped this pass).

Pattern: `internal/client/client.go` + the `site_*`/`server_data_source.go` files are the canonical exemplars new entities follow. Single-resource read paths come from the list response's `links.self` (not always `{parent}/{group}/{id}`); writes send a flat JSON body; only scalar attributes are mapped.

> [!NOTE]
> The API shape is **verified against the live API (2026-06-06)**: it's strict JSON:API — single resource is `{"data": {"id": "<string>", "type": "servers", "attributes": {...}}}`, with the real fields under `attributes` (where `id` is numeric). Org-scoped paths confirmed (`/api/orgs/{org}/servers/{id}`, org slug `kirchdev`). Auth is `Authorization: Bearer <token>`.

## Commands

Go (via `GNUmakefile`; needs Go ≥ 1.25 — `terraform-plugin-framework v1.19` requires it):

| Command        | What it does                                                       |
| :------------- | :----------------------------------------------------------------- |
| `make build`   | `go build -o terraform-provider-laravelforge .`                    |
| `make tidy`    | `go mod tidy` (refresh deps + `go.sum`)                            |
| `make vet`     | `go vet ./...`                                                      |
| `make fmt`     | `gofmt -s -w .`                                                    |
| `make test`    | `go test ./...`                                                    |
| `make testacc` | `TF_ACC=1 go test ./...` — hits the real Forge API, needs `FORGE_TOKEN` |

Node meta layer: `pnpm install` (wires husky hooks), `pnpm check` / `pnpm check:fix` (oxlint + oxfmt over JSON/YAML/MD — the CI gate for non-Go files).

### Manual smoke test (loads the binary in OpenTofu)

```bash
make build
cat > /tmp/lf.tfrc <<EOF
provider_installation {
  dev_overrides { "kirchdev/laravelforge" = "$(pwd)" }
  direct {}
}
EOF
TF_CLI_CONFIG_FILE=/tmp/lf.tfrc tofu -chdir=path/to/example validate
```

`validate` exercises the schema without calling the API; `plan`/`apply` would call the data source's `Read` and need a real `FORGE_TOKEN`.

## Conventions

- **Conventional Commits enforced** via commitlint on `git commit`. Don't `--no-verify` unless explicitly asked.
- **release-please** owns versioning + CHANGELOG + the release tag. During the Go conversion, switch `release-type` from `node` to `simple` (or `go`) and let the tag trigger a goreleaser workflow that builds + **GPG-signs** the provider artifacts.
- **Pre-1.0.** Schemas/behaviour may change; the consuming infra repo pins this provider **exactly**.
- **House style** for READMEs/meta files: the `/write-readme` skill encodes it — centered hero block, prescribed section emojis (✨ 📦 🚀 🤝 🛣️ 📄), GitHub callouts (`> [!TIP]`), license footer `[MIT](LICENSE) © [Titus Kirch](https://github.com/TitusKirch/) / [IT-Dienstleistungen Titus Kirch](https://kirch.dev)`.

## Build approach (summary — full detail in TEMP_AI.md)

1. **Generate** a Go API client + TF Framework schemas from Forge's OpenAPI spec (`terraform-plugin-codegen-openapi` → `-framework`). This is the lever — it's what `madewithlove/laravelforge` half-did and abandoned.
2. **Write the CRUD glue** per resource and **register** each in `provider.go` `Resources()`/`DataSources()` — the exact step the upstream stub never completed (its `Resources()` returns an empty slice).
3. **Vertical slice first:** `laravelforge_server` + `laravelforge_site` end-to-end with one acceptance test, then expand resource by resource.

## Don't relitigate

The decision to build our own provider (rather than use `madewithlove/laravelforge` — a non-functional stub — or `tonning/laravelforge` — working but thin and ~18 mo unmaintained) and the naming/registry choices are settled. Background lives in the infra repo's `tf-service-expansion-eval` memory and in `TEMP_AI.md`.
