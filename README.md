<div align="center">

# 🏗️ terraform-provider-laravelforge

**Manage your [Laravel Forge](https://forge.laravel.com) estate as code — servers, sites, SSL, daemons & more, reconciled by OpenTofu**

[![Status: experimental](https://img.shields.io/badge/status-experimental-f59e0b?style=flat-square)](https://github.com/kirchDev/terraform-provider-laravelforge)
[![CI](https://img.shields.io/github/actions/workflow/status/kirchDev/terraform-provider-laravelforge/ci.yml?branch=main&style=flat-square&label=ci)](https://github.com/kirchDev/terraform-provider-laravelforge/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-10b981?style=flat-square)](LICENSE)

</div>

---

```hcl
resource "laravelforge_site" "app" {
  server_id = laravelforge_server.web.id
  domain    = "app.example.com"
}
```

That's it. Laravel Forge servers, sites, SSL and daemons declared in HCL and reconciled by OpenTofu — not clicked together in a dashboard.

> [!IMPORTANT]
> **Early development, pre-1.0.** The provider is being built out resource by resource and is **not yet published** to the OpenTofu registry. Schemas and behaviour may change without notice until `v1.0.0`. See the Roadmap section below for current coverage.

## ✨ Features

- **🏗️ Forge as code** — declare servers, sites, databases, SSL certificates, daemons and scheduled jobs in HCL instead of the dashboard.
- **🧩 Generated from the OpenAPI** — resource schemas are derived from Forge's official API spec, so coverage tracks the upstream API.
- **🚀 OpenTofu-native** — published as `kirchdev/laravelforge`; works with Terraform too.
- **🔐 Simple auth** — a single Forge API token via the `token` argument or `FORGE_TOKEN`.
- **⚡ Modern stack** — built on the `terraform-plugin-framework` (not legacy SDKv2).

## 📦 Installation

```hcl
terraform {
  required_providers {
    laravelforge = {
      source  = "kirchdev/laravelforge"
      version = "~> 0.1"
    }
  }
}
```

> [!NOTE]
> Until the first registry release lands, install via a local/network mirror or a `dev_overrides` block pointing at a locally built binary.

## 🚀 Setup

```hcl
provider "laravelforge" {
  token = var.forge_token # or set FORGE_TOKEN in the environment
}

# Read an existing server
data "laravelforge_server" "web" {
  organization = "your-org"
  id           = 123456
}

# Manage a site on it
resource "laravelforge_site" "app" {
  organization = "your-org"
  server_id    = data.laravelforge_server.web.id
  type         = "php"
  name         = "app.example.com"
  php_version  = "php82"
}
```

Generate a Forge API token under **Forge → Account → API**, then export it:

```bash
export FORGE_TOKEN="forge_xxx"
tofu plan
```

## 🗺️ Coverage

**~57 resources + ~83 data sources** — every manageable entity of the new
org-scoped Forge API (pure action endpoints like reboot/restart/deploy-trigger
are intentionally excluded). All schemas compile and load (`tofu validate`);
**reads** are verified against the live API.

- **Servers** — server, PHP versions & config (cli/fpm/pool/opcache, max upload/exec), network, logs.
- **Sites** — site, environment, deployments (script, key, push-to-deploy, history), commands, workers, redirect & security rules, SSL certificates, load balancing, webhooks, and integration toggles (`octane`, `horizon`, `pulse`, `reverb`, `laravel-scheduler`, `laravel-maintenance`, `inertia`).
- **Services** — daemons (background processes), scheduled jobs, firewall rules, databases, database users, database backups, nginx templates, monitors, SSH keys.
- **Organization** — teams, roles & permissions, recipes, storage providers, server credentials, VPCs.
- **Read-only data sources** — providers / regions / sizes, organizations, current user, plus a singular + (where available) reads for everything above.

> [!IMPORTANT]
> **Writes (create/update/delete) are not yet acceptance-tested.** Routes and
> field shapes come from the live API + the official OpenAPI spec, but the
> verification token was read-only, so `Create`/`Update`/`Delete` need a
> write-scoped token to validate. Treat resources as **beta** until then.
> Only scalar attributes are mapped so far; nested object/array fields are a
> follow-up.

## 🔐 Security

Found a vulnerability? Please **don't** open a public issue — follow [SECURITY.md](SECURITY.md).

## 🤝 Contributing

PRs welcome. Conventional Commits required (enforced via commitlint). Husky runs the project's linters/formatters on `git commit`.

> [!TIP]
> Run `pnpm check:fix` before pushing — CI will catch what husky missed.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## 🛣️ Versioning

[Semantic Versioning](https://semver.org/) via [release-please](https://github.com/googleapis/release-please) — see [CHANGELOG.md](CHANGELOG.md).

## 📄 License

[MIT](LICENSE) © [Titus Kirch](https://github.com/TitusKirch/) / [IT-Dienstleistungen Titus Kirch](https://kirch.dev)
