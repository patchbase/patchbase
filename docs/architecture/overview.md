# Architecture overview

PatchBase is composed of a Go backend, a SvelteKit frontend, a PostgreSQL database, and a job queue. This page gives a high-level tour of how the pieces fit together.

## Components

```
                    ┌─────────────────────────────────┐
                    │       PatchBase Server            │
                    │  ┌──────────┐  ┌───────────────┐  │
  Browser  ◄──────► │  │ SvelteKit │  │  REST API     │  │
                    │  │ Dashboard │  │  (/api/v1/)   │  │
                    │  └──────────┘  └───────┬───────┘  │
                    │  ┌──────────────────────┴───────┐  │
                    │  │       Services Layer          │  │
                    │  │  Hosts · Advisories · Auth   │  │
                    │  │  SSH Pull · Matcher · Queue  │  │
                    │  └───────┬──────────┬──────────┘  │
                    └──────────┼──────────┼──────────────┘
                               │          │
                    ┌──────────▼──┐  ┌────▼─────────┐
                    │ PostgreSQL  │  │   River      │
                    │  (data)     │  │  (job queue) │
                    └─────────────┘  └──────────────┘

  Agent ◄──────────► POST /api/v1/agent/snapshots
```

## Server (`patchbase-server`)

The server is a Go binary built with:

- **Cobra** for CLI commands (`serve`, `migrate`, `version`)
- **Viper** for configuration (YAML with defaults)
- **net/http** with Go 1.22+ ServeMux pattern routing
- **pgxpool** for PostgreSQL connection pooling
- **River** (pgx-based) for background job queue
- **samber/do** for dependency injection
- **WebSocket** for real-time dashboard updates

The server embeds the SvelteKit dashboard as static files. In production, the same binary serves both the API and the frontend — no separate frontend deployment needed.

### Service layer

All business logic lives in `internal/services/`:

- **Hosts** — registration, approval, snapshot ingestion, SSH pull, manual report parsing
- **Advisory sync** — manifest fetching, SQLite download, advisory import, scope management
- **Matcher** — compares host packages against advisory rules (RPM-EVR and Debian version comparison)
- **Settings** — global SSH key management, email configuration
- **Auth** — JWT issuance and validation

### Job queue

River handles background jobs:

- **SSH pull jobs** — periodic SSH collection per host
- **Advisory sync jobs** — periodic advisory database refresh per scope

Jobs are inserted into PostgreSQL (River uses Postgres as its backing store) and workers process them concurrently.

## Agent (`patchbase-agent`)

The agent is a separate static Go binary with no runtime dependencies. It:

1. Reads `/etc/os-release` to detect the OS family
2. Collects package data using `dpkg-query` (APT) or `rpm -qa` (RPM)
3. Collects repository data from apt sources or dnf/yum repolist
4. Collects system metadata (hostname, machine-id, kernel, uptime)
5. Marshals everything into a protobuf `AgentSnapshot`
6. POSTs it to the server's `/api/v1/agent/snapshots` endpoint

The agent is stateless between runs — all configuration comes from `/etc/patchbase-agent/config.json`.

## Advisory database

Advisory data lives in per-scope SQLite databases hosted at `dl.patchbase.net`. Each scope (e.g., `rocky:9`, `ubuntu:jammy`) has its own database containing:

- `advisories` — advisory metadata (ID, severity, description, dates)
- `product_streams` — distribution channels (e.g., "Rocky Linux 10.2 BaseOS x86_64")
- `advisory_product_streams` — many-to-many link between advisories and streams
- `advisory_references` — external references (CVEs, vendor URLs)
- `affected_package_rules` — conditions for package vulnerability
- `fixed_packages` — specific package versions that fix the advisory

The server downloads these SQLite files, verifies checksums, and imports the records into PostgreSQL for querying.