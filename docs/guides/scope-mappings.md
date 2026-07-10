# Custom scope mappings

PatchBase maps each host to an advisory scope key based on its OS name, version, and architecture. The default mappings cover common distributions, but you can add your own to support custom setups or override the defaults.

## How scope keys work

A scope key identifies a specific advisory database. For example:

- `ubuntu:jammy` — Ubuntu 22.04 (Jammy) advisories
- `ubuntu:noble` — Ubuntu 24.04 (Noble) advisories
- `ubuntu:resolute` — Ubuntu 26.04 (Resolute) advisories
- `debian:bookworm-dsa` — Debian 12 (Bookworm) DSA advisories
- `rocky:9` — Rocky Linux 9 advisories
- `alma:9` — AlmaLinux 9 advisories

When a host sends a snapshot, the server resolves its scope key by checking each mapping's match rules against the host's OS family, name, version, major version, and architecture. The first match wins.

## Default mappings

PatchBase ships with these mappings:

| Match | Scope |
|-------|-------|
| Ubuntu 22.04 | `ubuntu:jammy` |
| Ubuntu 24.04 | `ubuntu:noble` |
| Ubuntu 26.04 | `ubuntu:resolute` |
| Debian GNU/Linux 12 | `debian:bookworm-dsa` |
| Debian GNU/Linux 13 | `debian:trixie-dsa` |
| Rocky Linux 9 | `rocky:9` |
| Rocky Linux 10 | `rocky:10` |
| AlmaLinux 9 | `alma:9` |
| AlmaLinux 10 | `alma:10` |

## Adding custom mappings

If you have a custom setup (for example, a mirrored repository with its own advisory feed), you can define custom mappings in `config.yaml`:

```yaml
advisory_sync:
  scope_mappings:
    - scope: "myorg:rocky-9-staging"
      match:
        os_name: "Rocky Linux"
        os_major: 9
        architecture: "x86_64"
    - scope: "ubuntu:noble"
      match:
        os_name: "Ubuntu"
        os_version: "24.04"
```

### Match rules

Each mapping has a `match` block with optional fields:

| Field | Description |
|-------|-------------|
| `os_family` | `apt` or `rpm` |
| `os_name` | Full or partial OS name (case-insensitive substring match) |
| `os_version` | Full or partial version string (case-insensitive substring match) |
| `os_major` | Major version as an integer |
| `architecture` | `x86_64`, `aarch64`, etc. |

All specified fields must match for the mapping to apply. Empty or omitted fields are ignored (i.e., they match anything).

## Overriding defaults

If you provide any custom `scope_mappings`, they are checked **first**. If none of your custom mappings match, the defaults are used as a fallback. This means you can override specific distributions without having to re-list every default.