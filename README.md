# PatchBase

Self-hosted vulnerability and patch management for Linux servers.

PatchBase collects installed package inventories from your hosts, matches them against upstream security advisories, and gives you a single dashboard to see what needs attention — without sending any data outside your network.

## Why?

Keeping track of security advisories across a fleet of Linux machines is tedious. You end up with scripts that are half-maintained, spreadsheets that are always out of date, and no clear answer to the question: *"which of my hosts are affected by CVE-2024-XXXXX?"*

PatchBase was built to solve that.

## Features

- **Continuous visibility** — snapshots of installed packages, kernels, and repositories from every host, collected on a schedule
- **Advisory matching** — package inventories matched against curated security advisories from Ubuntu, Debian, Rocky Linux, and AlmaLinux
- **Self-hosted** — runs on your infrastructure, no data leaves your network
- **Three onboarding modes** — install the agent, let the server SSH in, or run a script manually

## Supported operating systems

| Family | Distributions |
|--------|--------------|
| APT | Ubuntu 22.04 / 24.04 / 26.04, Debian 12 / 13 |
| RPM | Rocky Linux 9 / 10, AlmaLinux 9 / 10 |

Both `x86_64` and `aarch64` architectures are supported.

## Quick start

The fastest way to try PatchBase is with Docker Compose:

```bash
git clone https://github.com/patchbase/patchbase.git
cd patchbase
docker compose up -d
```

Then build and run the server:

```bash
bazel run //cmd/patchbase-server -- migrate
bazel run //cmd/patchbase-server -- serve
```

Open `http://localhost:5199` in your browser and follow the setup wizard.

For production deployments, see the [installation guide](https://docs.patchbase.net/installation/requirements).

## Architecture

```
                    ┌─────────────────────────────────────────┐
                    │          PatchBase Server               │
                    │   ┌────────────┐  ┌───────────────┐     │
  Browser  ◄──────► │   │ SvelteKit  │  │  REST API     │     │
                    │   │ Dashboard  │  │  (/api/v1/)   │     │
                    │   └────────────┘  └───────┬───────┘     │
                    │   ┌───────────────────────┴─────────┐   │
                    │   │        Services Layer           │   │
                    │   │   Hosts · Advisories · Auth     │   │
                    │   │   SSH Pull · Matcher · Queue    │   │
                    │   └─────────┬──────────┬────────────┘   │
                    └────────────-┼──────────┼────────────────┘
                                  │          │
                    ┌────────────▼──┐   ┌────▼─────────┐
                    │   PostgreSQL  │   │   River      │
                    │     (data)    │   │  (job queue) │
                    └───────────────┘   └──────────────┘

  Agent ◄──────────► POST /api/v1/agent/snapshots
```

**Server** — Go binary serving a SvelteKit dashboard and REST API, backed by PostgreSQL and a River job queue.

**Agent** — lightweight static Go binary installed on each host. Collects package data and reports back on a systemd timer.

**Advisory database** — per-distribution SQLite databases of security advisories, automatically downloaded and matched against your hosts.

## Documentation

Full documentation is available at **[docs.patchbase.net](https://docs.patchbase.net)**, including:

- [Installation](https://docs.patchbase.net/installation/requirements) — RPM/DEB packages, Docker Compose, building from source
- [Onboarding hosts](https://docs.patchbase.net/onboarding/overview) — agent, SSH pull, and manual modes
- [Configuration reference](https://docs.patchbase.net/configuration/server) — server and agent config
- [API reference](https://docs.patchbase.net/api/overview) — REST API and WebSocket events
- [Architecture](https://docs.patchbase.net/architecture/overview) — internals and design decisions
- [Development](https://docs.patchbase.net/development/building) — building, testing, contributing

## Building from source

PatchBase uses [Bazel](https://bazel.build) as its build system. You'll need [bazelisk](https://github.com/bazelbuild/bazelisk) to manage the Bazel version automatically.

```bash
# Build everything
bazel build //...

# Build just the server
bazel build //cmd/patchbase-server

# Build the documentation site
bazel build //docs:build_docs
```

See the [development setup guide](https://docs.patchbase.net/development/dev-setup) for details.

## Contributing

Pull requests are welcome. For significant changes, please open an issue first to discuss what you have in mind.

See the [contributing guide](https://docs.patchbase.net/contributing) for details.

## License

[Apache License 2.0](LICENSE)
