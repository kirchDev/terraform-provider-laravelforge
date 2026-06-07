# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A first-party **OpenTofu / Terraform provider for [Laravel Forge](https://forge.laravel.com)**, owned by kirchDev. It manages Forge resources (servers, sites, databases, SSL certificates, daemons, scheduled jobs, …) as code so the Forge estate can live in the same IaC workflow as the rest of the kirchDev infrastructure.

- **Provider type (HCL):** `laravelforge` → `provider "laravelforge" {}`
- **OpenTofu registry address:** `kirchdev/laravelforge`
- **Go module:** `github.com/kirchDev/terraform-provider-laravelforge`
- **SDK:** `terraform-plugin-framework` (NOT the legacy SDKv2)

The repo name format `NAMESPACE/terraform-provider-NAME` is **mandatory** for the OpenTofu/Terraform registry — it's not a style choice.

## Current state

Two layers coexist:

- **Node meta layer** (from the `scaffold` template): pnpm + oxlint + oxfmt + husky + commitlint + release-please + CI/CodeQL/Dependabot + issue/PR templates. Gates config/docs (JSON/YAML/MD).
- **Go provider**: `go.mod` (`terraform-plugin-framework`), `main.go`, `internal/client/` (generic JSON:API client: `Get`/`List`/`Write`/`Delete`/`NotFound`), and `internal/provider/` with **~57 resources + ~83 data sources** (141 files) covering essentially the full manageable surface of the org-scoped API (servers + PHP/network/logs; sites + env/deployments/commands/workers/rules/SSL/load-balancing/webhooks/integration-toggles; databases + users + backups; daemons, scheduled jobs, firewall rules, nginx templates, monitors, SSH keys; recipes, teams, roles & permissions, storage providers, server credentials, VPCs; read-only data sources for providers/regions/sizes, organizations, current user, …). Pure action endpoints (reboot/restart/deploy-trigger) are intentionally excluded.

Verified: `go build ./...`, `go vet ./...`, `gofmt` clean, `tofu validate` green across all ~140 schemas, and **reads** against the live API. Tests: `internal/client/client_test.go` unit-tests the client transport (flat-body writes, JSON:API parse, `NotFound`/`APIError`) against an `httptest` mock, and `TF_ACC=1` acceptance tests (`*_resource_test.go` + `acctest_helpers_test.go`) drive the structure-representative resources through a full apply/refresh/import/destroy cycle against an in-memory Forge mock. Docs: 141 pages under `docs/`, rendered by `make docs` (tfplugindocs). Release plumbing is in place: `.goreleaser.yml`, `.github/workflows/release.yml`, release-please `go` type, `terraform-registry-manifest.json`.

> [!IMPORTANT]
> **Write paths (Create/Update/Delete) are not verified against the _live_ API** — Forge tokens scope by ability, not per-resource, so a write token would be broad and live tests would create real, sometimes costly, infra. They are verified offline instead: the client transport contract via `client_test.go`, and the per-resource CRUD glue (body fields, PUT vs PATCH, paths, the daemon/scheduled_job quirks, import) via `TF_ACC=1` acceptance tests against an in-memory mock, for the structure-representative resources. Only **scalar** attributes are mapped; nested object/array fields are not.

## API shape (verified against the live API, 2026-06)

- Base `https://forge.laravel.com`; everything under `/api/orgs/{org}/...`. Org slug `kirchdev`. Auth `Authorization: Bearer <token>`.
- **Reads are JSON:API**: a single resource is `{"data":{"id":"<string>","type":"...","attributes":{...}}}` — the real fields live under `attributes` (where `id` is numeric) and the resource-level `id` is a string.
- **Writes are FLAT JSON** (a map), NOT JSON:API; the update method varies (PUT vs PATCH per entity).
- **The single-resource read path comes from the list response's `links.self`** — not always `{parent}/{group}/{id}` (e.g. sites read org-level `/api/orgs/{org}/sites/{id}` but create/update/delete server-scoped).
- HCL reserves `provider`, so the API `provider` field is exposed as `cloud_provider`.
- OpenAPI spec: `GET /api/docs.openapi` (copied to `.forge-openapi.json`, gitignored — the codegen/field source of truth).

## Commands

Go (via `GNUmakefile`; needs **Go ≥ 1.25** — `terraform-plugin-framework v1.19` requires it):

| Command         | What it does                                                                  |
| :-------------- | :---------------------------------------------------------------------------- |
| `make build`    | `go build -o terraform-provider-laravelforge .`                               |
| `make tidy`     | `go mod tidy` (refresh deps + `go.sum`)                                       |
| `make fmt`      | `gofmt -s -w .`                                                               |
| `make vet`      | `go vet ./...`                                                                |
| `make lint`     | `golangci-lint run`                                                           |
| `make generate` | `go generate ./...`                                                           |
| `make docs`     | render `docs/` from the schema (build + export + tfplugindocs)                |
| `make test`     | `go test ./...`                                                               |
| `make testacc`  | `TF_ACC=1 go test ./...` — mock acceptance tests; needs a TF binary, no token |

Node meta layer: `pnpm install` (wires husky hooks), `pnpm check` / `pnpm check:fix` (oxlint + oxfmt). CI (`.github/workflows/ci.yml`, on PRs to `main`) runs a **Go job** (build·vet·gofmt·test + `TF_ACC` mock acceptance tests, with OpenTofu installed) and a **Lint job** (oxlint + oxfmt).

> [!NOTE]
> Generated files are excluded from oxfmt via `.prettierignore` (`docs/` from tfplugindocs, `CHANGELOG.md` from release-please). Don't reformat them — the next generation undoes it.

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

## Patterns & gotchas

- `internal/client/client.go` — generic JSON:API client: `Get`, `List`, `Write(method, path, bodyMap, &attrs)`, `Delete`, `client.NotFound(err)`, `APIError`.
- Exemplars new entities follow: `site_resource.go` (CRUD + plan modifiers + `ImportState` + `readInto`), `site_data_source.go`, `server_data_source.go`.
- Each entity is its own file(s) with one `*Attributes` struct (scalars only); constructors `New{Camel}Resource` / `New{Camel}DataSource`; TypeName `laravelforge_{snake}`; all registered in `provider.go`.
- **daemon** → the API group is `background-processes`, not `daemons`.
- **scheduled_job** → no update endpoint (all `RequiresReplace`); the request `frequency` is lowercase but the response Capitalizes it → don't refresh it from the API (drift).
- **site** → read is org-level; create/update/delete are server-scoped; update is PUT.
- firewall/redirect/security rules → the read-only discovery token lacked scope; routes were confirmed via 405/OpenAPI.

## Release & publishing

- **release-please** (`.github/workflows/release-please.yml`, `release-type: go`) owns versioning + CHANGELOG + the tag + GitHub release. It runs on `main` with a **GitHub App token** (minted from a Bitwarden-stored PEM).
- When it cuts a release (`release_created == 'true'`), a second job in the **same workflow** runs **goreleaser**: builds the cross-platform archives, **GPG-signs** the checksums (key + passphrase fetched from **Bitwarden SM**, like the App PEM), and **appends** them to the release (`release.mode: append`). No separate tag-triggered workflow.
- The registry consumes the per-platform zips + `SHA256SUMS` + detached `.sig` + `manifest.json` (protocol `6.0`, from `terraform-registry-manifest.json`).
- Schemas/behaviour may still change; the consuming infra repo pins this provider **exactly**.

## Conventions

- **Conventional Commits enforced** via commitlint on `git commit`. Don't `--no-verify` unless explicitly asked.
- **House style** for READMEs/meta files: the `/write-readme` skill encodes it — centered hero block, prescribed section emojis (✨ 📦 🚀 🤝 🛣️ 📄), GitHub callouts (`> [!TIP]`), license footer `[MIT](LICENSE) © [Titus Kirch](https://github.com/TitusKirch/) / [IT-Dienstleistungen Titus Kirch](https://kirch.dev)`.

## Don't relitigate

The decision to build our own provider (rather than use `madewithlove/laravelforge` — a non-functional stub — or `tonning/laravelforge` — working but thin and ~18 mo unmaintained) and the naming/registry choices are settled. Background lives in the infra repo's `tf-service-expansion-eval` memory.
