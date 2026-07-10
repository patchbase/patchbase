# Advisory database sync

PatchBase maintains per-distribution SQLite databases of security advisories. The server downloads and updates these automatically. Here's how it works and how to control it.

## How sync works

The advisory sync process:

1. **Fetches a manifest** from the configured base URL (default: `https://dl.patchbase.net/v1/advisory-db/manifest.json`). The manifest lists all available scopes, their checksums, and download URLs.
2. **Checks if a download is needed** — if the local copy's SHA-256 matches the manifest, it skips the download.
3. **Downloads the SQLite database** to a staging directory, verifies the checksum, and atomically moves it into place.
4. **Imports the advisory records** into PostgreSQL — advisories, references, product streams, affected package rules, and fixed packages.
5. **Re-matches hosts** that belong to the updated scope, so vulnerability status reflects the latest advisories.

## Scope demand

Advisory databases are only synced when they're needed. When a host sends a snapshot, the server resolves its advisory scope key (e.g., `rocky:9`, `ubuntu:jammy`). If that scope isn't in the database yet, it's registered as "pending" and a sync job is queued.

This means the first host you onboard for a given distribution will trigger a one-time advisory database download. Subsequent hosts in the same scope use the already-synced data.

## Sync schedule

After the initial sync, the server periodically checks for updates:

- **Default interval:** every 6 hours
- **Configurable** via `advisory_sync.refresh_interval` in the server config

During each periodic check, the server fetches the manifest and compares checksums. If nothing changed, it's a no-op. If a new version is available, it downloads and imports it.

## Manual sync

You can trigger a manual sync from the dashboard. Go to **Advisories → Scopes**, find the scope you want to update, and click **Sync now**. This is useful when you know a new advisory was published and want to pick it up immediately.

## Scope statuses

Each scope can be in one of these states:

| Status | Meaning |
|--------|---------|
| `pending` | Scope is registered but hasn't been synced yet |
| `syncing` | Download and import in progress |
| `synced` | Advisory database is up to date |
| `failed` | Last sync attempt failed (check the error message) |

The scope list shows the last sync time, last success time, advisory count, and file size for each scope.

## Configuration

The advisory sync is configured in the server's `config.yaml`:

```yaml
advisory_sync:
  base_url: "https://dl.patchbase.net/v1/advisory-db"
  refresh_interval: 6h
  storage_dir: "/var/lib/patchbase-server/db/advisories"
```

See the [configuration reference](../configuration/server) for all options.