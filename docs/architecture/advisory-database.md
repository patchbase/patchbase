# Advisory database system

PatchBase maintains a curated collection of security advisories organized into per-distribution "scopes." This page explains the data model and sync pipeline.

## Scope structure

A scope represents a set of advisories for a specific distribution channel. Examples:

| Scope key | Distribution |
|-----------|-------------|
| `ubuntu:jammy` | Ubuntu 22.04 |
| `ubuntu:noble` | Ubuntu 24.04 |
| `ubuntu:resolute` | Ubuntu 26.04 |
| `debian:bookworm-dsa` | Debian 12 |
| `debian:trixie-dsa` | Debian 13 |
| `rocky:9` | Rocky Linux 9 |
| `rocky:10` | Rocky Linux 10 |
| `alma:9` | AlmaLinux 9 |
| `alma:10` | AlmaLinux 10 |

Each scope has its own SQLite database file hosted at `dl.patchbase.net`.

## SQLite schema

Each scope database contains these tables:

### `advisories`

| Column | Description |
|--------|-------------|
| `id` | Advisory identifier (e.g., `RHSA-2024:1234`, `USN-6021-1`) |
| `source_system` | Source (e.g., `rocky`, `ubuntu`) |
| `raw_source_id` | Original ID from the upstream source |
| `source_url` | Link to the original advisory |
| `vendor` | Vendor name |
| `advisory_type` | Type (e.g., `security`, `bugfix`) |
| `severity` | Severity rating (critical, important, moderate, low) |
| `summary` | Short description |
| `description` | Full description |
| `published_at` | Publication timestamp |
| `updated_at` | Last update timestamp |
| `evidence_tier` | Evidence quality level |
| `is_security` | Whether this is a security advisory |

### `product_streams`

Represents a distribution channel (e.g., "Rocky Linux 10.2 BaseOS x86_64"). Each stream has a vendor, distribution family, major/minor version, architecture, repo family, and optional CPE.

### `advisory_product_streams`

Many-to-many link between advisories and product streams.

### `advisory_references`

External references (CVE IDs, vendor advisories, CVSS scores).

### `affected_package_rules`

Conditions that determine if a package is vulnerable. Each rule specifies:

- Package name
- Source RPM (for RPM systems)
- Architecture constraint
- Version constraints (epoch, version, release)
- RPM EVR comparison rules
- Context (e.g., base, appstream)

### `fixed_packages`

Specific package versions (NEVRA) that resolve the advisory.

## Sync pipeline

```
1. Fetch manifest.json from base_url
2. Find scope detail in manifest
3. Compare SHA-256 checksum with local copy
   ├── Same hash + already imported → skip download, re-match hosts
   └── Different hash or missing → continue
4. Download SQLite file to staging directory
5. Verify SHA-256 checksum
6. Atomically rename to final path
7. Open SQLite, verify advisories table exists
8. Begin Postgres transaction
9. Import records:
   a. Product streams (upsert)
   b. Clean up old streams for this scope
   c. Advisories (upsert)
   d. Advisory references (insert)
   e. Advisory ↔ product stream links (insert)
   f. Affected package rules (insert)
   g. Fixed packages (insert)
10. Commit transaction
11. Re-match all hosts in this scope
12. Update scope status to "synced"
13. Clean up old SQLite file if hash changed
```

The import is transactional — if any step fails, the entire import is rolled back and the scope is marked as `failed` with the error message preserved.

## Manifest format

The manifest is a JSON document listing all available scopes:

```json
{
  "schema_version": 1,
  "generated_at": "2025-01-15T00:00:00Z",
  "scopes": [
    {
      "key": "rocky:9",
      "path": "rocky/9/advisories.db",
      "url": "https://dl.patchbase.net/v1/advisory-db/rocky/9/advisories.db",
      "sha256": "abc123...",
      "size_bytes": 4567890,
      "updated_at": "2025-01-14T18:00:00Z",
      "advisory_count": 1247,
      "license_feature": ""
    }
  ]
}
```

The server fetches this manifest on every sync cycle to detect changes.