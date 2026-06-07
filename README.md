<div align="center">

# 🛠️ terraform-provider-laravelforge

**Manage your entire [Laravel Forge](https://forge.laravel.com) estate as code — servers, sites, databases, daemons, SSL & more, reconciled by OpenTofu**

[![Status: beta](https://img.shields.io/badge/status-beta-f59e0b?style=flat-square)](https://github.com/kirchDev/terraform-provider-laravelforge)
[![CI](https://img.shields.io/github/actions/workflow/status/kirchDev/terraform-provider-laravelforge/ci.yml?branch=main&style=flat-square&label=ci)](https://github.com/kirchDev/terraform-provider-laravelforge/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-10b981?style=flat-square)](LICENSE)

</div>

---

```hcl
resource "laravelforge_site" "app" {
  organization = "your-org"
  server_id    = laravelforge_server.web.id
  type         = "php"
  name         = "app.example.com"
}
```

That's it. Servers, sites, databases, daemons and SSL declared in HCL and reconciled by OpenTofu — not clicked together in a dashboard.

> [!IMPORTANT]
> **Pre-1.0 / beta.** Built against the new org-scoped Forge JSON:API. All schemas load and **reads are verified against the live API**, but **writes (create/update/delete) are not yet acceptance-tested** and only scalar attributes are mapped — pin an exact version and test before relying on it. Not yet on the OpenTofu registry.

## 📦 Install & run

```hcl
terraform {
  required_providers {
    laravelforge = {
      source  = "kirchdev/laravelforge"
      version = "~> 0.1"
    }
  }
}

provider "laravelforge" {
  token = var.forge_token # or set FORGE_TOKEN
}

resource "laravelforge_site" "app" {
  organization = "your-org"
  server_id    = data.laravelforge_server.web.id
  type         = "php"
  name         = "app.example.com"
  php_version  = "php82"
}
```

```bash
export FORGE_TOKEN="forge_xxx"   # Forge → Account → API
tofu plan
```

> [!NOTE]
> Until the first registry release, install via a local/network mirror or a `dev_overrides` block pointing at a locally built binary (`make build`).

## ✨ Features

- **🏗️ Forge as code** — servers, sites, databases, daemons, scheduled jobs, SSL and more in HCL.
- **🧩 Full API coverage** — ~57 resources + ~83 data sources across essentially every manageable Forge entity.
- **🚀 OpenTofu-native** — `kirchdev/laravelforge`; Terraform-compatible.
- **🔐 Simple auth** — one Forge token via `token` or `FORGE_TOKEN`.
- **⚡ Modern stack** — `terraform-plugin-framework`; docs generated from the schema.

## 🗺️ Coverage

~57 resources + ~83 data sources covering essentially every manageable Forge entity (pure actions like reboot / restart / deploy-trigger are intentionally excluded).

<details>
<summary>Full coverage</summary>

- **Servers** — server, PHP versions & config (cli/fpm/pool/opcache, max upload & execution), network, logs.
- **Sites** — site, environment, deployments (script, key, push-to-deploy, history), commands, workers, redirect & security rules, SSL, load balancing, webhooks, and integration toggles (`octane`, `horizon`, `pulse`, `reverb`, `laravel-scheduler`, `laravel-maintenance`, `inertia`).
- **Services** — daemons (background processes), scheduled jobs, firewall rules, databases, database users, database backups, nginx templates, monitors, SSH keys.
- **Organization** — teams, roles & permissions, recipes, storage providers, server credentials, VPCs.
- **Data sources** — a read for everything above plus providers / regions / sizes, organizations and the current user.

</details>

## 📚 Documentation

Per-resource docs live under [`docs/`](docs/), generated from the schema with `make docs` (build + export schema + tfplugindocs).

## 🤝 Contributing

PRs welcome. Conventional Commits required (enforced via commitlint). Husky runs the linters/formatters on `git commit`.

> [!TIP]
> Run `make build && go vet ./...` before pushing — CI will catch what husky missed.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## 🛣️ Versioning

[Semantic Versioning](https://semver.org/) via [release-please](https://github.com/googleapis/release-please) — see [CHANGELOG.md](CHANGELOG.md).

## 📄 License

[MIT](LICENSE) © [Titus Kirch](https://github.com/TitusKirch/) / [IT-Dienstleistungen Titus Kirch](https://kirch.dev)
