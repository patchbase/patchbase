# Advisory endpoints

## List advisory scopes

```http
GET /api/v1/advisories/scopes
Authorization: Bearer <jwt>
```

Returns the status of all advisory scopes known to the server, including sync status, advisory counts, and host usage.

### Response (200)

```json
[
  {
    "scope_key": "rocky:9",
    "status": "synced",
    "last_sync_at": "2025-01-15T06:00:00Z",
    "last_success_at": "2025-01-15T06:00:00Z",
    "advisory_count": 1247,
    "sha256": "abc123...",
    "size_bytes": 4567890,
    "host_usage_count": 5,
    "next_refresh_at": "2025-01-15T12:00:00Z"
  }
]
```

## Trigger manual sync

```http
POST /api/v1/advisories/scopes/{scopeKey}/sync
Authorization: Bearer <jwt>
```

Queues an advisory database sync job for the specified scope. If a sync is already running or scheduled, the request is ignored (the job is deduplicated by arguments and state).

## Advisory overview

```http
GET /api/v1/advisories/overview
Authorization: Bearer <jwt>
```

Returns aggregate counts across all scopes.

### Response (200)

```json
{
  "total_advisories": 5000,
  "total_scopes": 4,
  "synced_scopes": 3
}
```

## Get advisory details

```http
GET /api/v1/advisories/{id}
Authorization: Bearer <jwt>
```

Returns details for a single advisory, including references, affected package rules, and fixed packages.

### Response (200)

```json
{
  "id": "RHSA-2024:1234",
  "source_system": "rocky",
  "vendor": "rocky",
  "advisory_type": "security",
  "severity": "important",
  "summary": "Important: openssl security update",
  "description": "...",
  "published_at": "2024-06-15T00:00:00Z",
  "updated_at": "2024-06-15T00:00:00Z",
  "evidence_tier": "vendor",
  "is_security": true
}
```