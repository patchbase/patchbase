# Host endpoints

## List hosts

```http
GET /api/v1/hosts
Authorization: Bearer <jwt>
```

Returns an array of host objects with current state information.

### Response (200)

```json
[
  {
    "id": "h_xxx",
    "onboarding_mode": "agent",
    "approval_status": "approved",
    "display_name": "web-01",
    "hostname": "web-01.example.com",
    "os_family": "rpm",
    "os_name": "Rocky Linux",
    "os_major": 10,
    "os_version": "10.2",
    "architecture": "x86_64",
    "status": "active",
    "overall_action": "none",
    "critical_count": 0,
    "important_count": 2,
    "moderate_count": 5,
    "available_updates": 12,
    "needs_reboot": 0,
    "needs_restart": 0,
    "last_seen_at": "2025-01-15T10:00:00Z",
    "created_at": "2025-01-01T00:00:00Z"
  }
]
```

## Get host details

```http
GET /api/v1/hosts/{hostID}
Authorization: Bearer <jwt>
```

## Get latest snapshot

```http
GET /api/v1/hosts/{hostID}/snapshot
Authorization: Bearer <jwt>
```

Returns the most recent snapshot for the host, including the raw protobuf payload.

## List vulnerable packages

```http
GET /api/v1/hosts/{hostID}/packages/vulnerable
Authorization: Bearer <jwt>
```

Returns packages on the host that are matched against security advisories, with the advisory details and available fix versions.

## List upgradable packages

```http
GET /api/v1/hosts/{hostID}/packages/upgradable
Authorization: Bearer <jwt>
```

Returns packages with newer versions available in the host's enabled repositories.

## Get kernel posture

```http
GET /api/v1/hosts/{hostID}/kernel-posture
Authorization: Bearer <jwt>
```

Returns information about whether the running kernel matches the latest installed kernel (i.e., whether a reboot is needed to apply a kernel update).

## Create SSH host

```http
POST /api/v1/hosts/ssh
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "display_name": "db-01",
  "hostname": "db-01.example.com",
  "ssh_user": "monitor",
  "frequency_minutes": 360,
  "unique_key_pair": true
}
```

Response includes the generated public key for installation on the target host.

## Create manual host

```http
POST /api/v1/hosts/manual
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "display_name": "air-gapped-host",
  "hostname": "air-gapped-host"
}
```

## Get collector script

```http
GET /api/v1/hosts/manual/script?os_family=apt
Authorization: Bearer <jwt>
```

Returns the collector script for the specified OS family (`apt` or `rpm`).

## Upload manual report

```http
POST /api/v1/hosts/{hostID}/report
Authorization: Bearer <jwt>
Content-Type: text/plain

<report content>
```

Parses the report, creates a snapshot, and runs advisory matching.

## Approve host

```http
POST /api/v1/hosts/{hostID}/approve
Authorization: Bearer <jwt>
```

## Delete host

```http
DELETE /api/v1/hosts/{hostID}
Authorization: Bearer <jwt>
```

Removes the host and all associated data (snapshots, pull jobs, tokens).

## Trigger SSH pull

```http
POST /api/v1/hosts/{hostID}/pull-now
Authorization: Bearer <jwt>
```

Triggers an immediate SSH pull collection job for the host.

## List pull jobs

```http
GET /api/v1/hosts/{hostID}/pull-jobs
Authorization: Bearer <jwt>
```

Returns the last 10 SSH pull job runs with status, timestamps, and error messages.