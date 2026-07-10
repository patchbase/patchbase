---
slug: /
---

# PatchBase

PatchBase is a self-hosted vulnerability and patch management platform for Linux servers. It collects installed package inventories from your hosts, matches them against upstream security advisories, and gives you a single dashboard to see what needs attention.

## Why PatchBase?

Keeping track of security advisories across a fleet of Linux machines is tedious. You end up with scripts that are half-maintained, spreadsheets that are always out of date, and no clear answer to the question: *"which of my hosts are affected by CVE-2024-XXXXX?"*

PatchBase was built to solve that. It gives you:

- **Continuous visibility** — snapshots of installed packages, kernels, and repositories from every host, collected on a schedule.
- **Advisory matching** — your package inventories are matched against curated security advisories from your distributions (Ubuntu, Debian, Rocky Linux, AlmaLinux).
- **Self-hosted** — runs on your infrastructure. No data leaves your network.
- **Three onboarding modes** — install the agent, let the server SSH in, or run a script manually. Whatever fits your environment.

## How it works

PatchBase has three main components:

### The server

The PatchBase server is a Go binary that serves a web dashboard and a REST API. It stores host inventory data and advisory records in PostgreSQL, and runs background jobs to collect snapshots from hosts and sync advisory databases.

### The agent

The PatchBase agent is a lightweight Go binary you install on each host. It collects a snapshot of the system — installed packages, enabled repositories, kernel version, OS information — and sends it to the server. The agent runs as a systemd timer by default, so it wakes up periodically, collects, and reports back.

### The advisory database

PatchBase maintains per-distribution SQLite databases of security advisories. The server downloads these automatically and matches them against your hosts' package inventories to determine which advisories affect which hosts.

## Supported operating systems

PatchBase currently supports two package families:

| Family | Distributions |
|--------|--------------|
| APT | Ubuntu, Debian |
| RPM | Rocky Linux, AlmaLinux |

Both `x86_64` and `aarch64` architectures are supported.

## Getting started

Head to the [Installation guide](./installation/requirements) to get the server running, or jump straight to [onboarding your first host](./onboarding/overview) once the server is up.