# Matching engine

The matching engine (`internal/services/matchers/`) is what turns raw package inventories and advisory databases into actionable vulnerability findings. This page explains how it works.

## The matching problem

Given:
- A host's installed packages (name, epoch, version, release, architecture)
- An advisory database with affected package rules and fixed packages

Determine which advisories affect which packages on the host.

This sounds simple but version comparison is tricky — RPM and Debian each have their own versioning schemes with epochs, tilde suffixes, and other edge cases.

## RPM version comparison

For RPM-based systems (Rocky, AlmaLinux), the matcher uses RPM's epoch-version-release (EVR) comparison algorithm:

1. **Epoch** — integer comparison (defaults to 0)
2. **Version** — RPM version string comparison (handles letters, digits, and tilde)
3. **Release** — RPM release string comparison

The matcher compares the installed package's EVR against each advisory's affected package rules. A rule typically specifies a version constraint like "version < 3.0.10-1" or an RPM EVR rule string.

### Implementation

The RPM matcher (`matcher_rpmver.go`) implements the RPM version comparison algorithm, including:

- Epoch parsing (with `(none)` handling)
- Version string tokenization (alternating digit and non-digit segments)
- Tilde (`~`) sorting (sorts before empty string)
- Caret (`^`) sorting (sorts after empty string, before tilde)

## Debian version comparison

For APT-based systems (Ubuntu, Debian), the matcher uses Debian's version comparison algorithm:

1. **Epoch** — integer comparison (prefix with `epoch:`)
2. **Upstream version** — Debian version comparison (digits, letters, `~`, `+`, `-`)
3. **Debian revision** — everything after the last `-`

### Implementation

The Debian matcher (`matcher_debianver.go`) handles:

- Epoch prefixes (`0:` through `9:`)
- Tilde (`~`) sorting (sorts before everything, including empty)
- Plus (`+`) sorting (sorts after letters but before tilde-prefixed strings)
- Hyphen-separated revision splitting

## Stream matching

Before comparing package versions, the matcher determines which product streams apply to the host. A product stream represents a specific distribution channel (e.g., "Rocky Linux 10.2 BaseOS x86_64").

The stream matcher (`matcher_streams.go`) filters advisory product streams by:

- Vendor (e.g., `rocky`, `alma`)
- Major version
- Architecture
- Repository family

Only advisories linked to matching product streams are considered for package-level matching.

## The MatchSnapshot flow

When a snapshot is ingested (via agent, SSH pull, or manual report), the matcher runs:

```
MatchSnapshot(hostID, snapshotID)
    │
    ├── Get host's advisory scope key
    ├── Get all advisories for that scope
    ├── Filter advisories by matching product streams
    ├── For each advisory:
    │   ├── Get affected package rules
    │   ├── Compare each host package against rules
    │   ├── Get fixed packages
    │   ├── Determine if host package version is:
    │   │   ├── Vulnerable (matches affected rule)
    │   │   ├── Fixed (matches a fixed package version)
    │   │   └── Unknown (can't determine)
    │   └── Record the match result
    ├── Aggregate counts by severity
    └── Update host current state
```

## Host current state

After matching, the host's state is updated with:

| Field | Description |
|-------|-------------|
| `overall_action` | `none`, `update_package`, or `reboot` |
| `critical_count` | Number of critical advisories affecting the host |
| `important_count` | Number of important advisories |
| `moderate_count` | Number of moderate advisories |
| `actionable_count` | Advisories with an available fix |
| `available_updates` | Packages with newer versions in repos |
| `needs_reboot` | Hosts where running kernel < installed kernel |
| `no_fix` | Advisories with no fixed package available |
| `unknown` | Advisories where matching couldn't determine status |

## Re-matching on advisory updates

When an advisory database is re-synced, `MatchHostsForScope` re-runs the matcher for all hosts in that scope. This means new advisories are reflected immediately — you don't have to wait for hosts to send new snapshots.