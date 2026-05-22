-- name: UpsertAdvisoryScope :one
INSERT INTO advisory_scopes (
    scope_key,
    status,
    last_sync_at,
    last_success_at,
    last_error,
    advisory_count,
    sha256,
    size_bytes,
    local_path,
    next_refresh_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (scope_key) DO UPDATE SET
    status = EXCLUDED.status,
    last_sync_at = COALESCE(EXCLUDED.last_sync_at, advisory_scopes.last_sync_at),
    last_success_at = COALESCE(EXCLUDED.last_success_at, advisory_scopes.last_success_at),
    last_error = EXCLUDED.last_error,
    advisory_count = EXCLUDED.advisory_count,
    sha256 = COALESCE(EXCLUDED.sha256, advisory_scopes.sha256),
    size_bytes = EXCLUDED.size_bytes,
    local_path = COALESCE(EXCLUDED.local_path, advisory_scopes.local_path),
    next_refresh_at = COALESCE(EXCLUDED.next_refresh_at, advisory_scopes.next_refresh_at),
    updated_at = now()
RETURNING *;

-- name: GetAdvisoryScope :one
SELECT * FROM advisory_scopes
WHERE scope_key = $1;

-- name: ListAdvisoryScopes :many
SELECT * FROM advisory_scopes
ORDER BY scope_key ASC;

-- name: ListAdvisoryScopeStats :many
SELECT
    as_scopes.scope_key,
    as_scopes.status,
    as_scopes.last_sync_at,
    as_scopes.last_success_at,
    as_scopes.last_error,
    as_scopes.advisory_count,
    as_scopes.sha256,
    as_scopes.size_bytes,
    as_scopes.local_path,
    as_scopes.next_refresh_at,
    COUNT(h.id)::int AS host_count
FROM advisory_scopes as_scopes
LEFT JOIN hosts h ON h.advisory_scope_key = as_scopes.scope_key
GROUP BY as_scopes.scope_key
ORDER BY as_scopes.scope_key ASC;

-- name: UpdateAdvisoryScopeStatus :one
UPDATE advisory_scopes
SET
    status = $2,
    last_error = $3,
    updated_at = now()
WHERE scope_key = $1
RETURNING *;

-- name: InsertAdvisoryScopeIfNotExists :exec
INSERT INTO advisory_scopes (
    scope_key,
    status
) VALUES (
    $1, $2
)
ON CONFLICT (scope_key) DO NOTHING;

-- name: UpsertAdvisoryScopeStatus :one
INSERT INTO advisory_scopes (
    scope_key,
    status,
    last_error
) VALUES (
    $1, $2, $3
)
ON CONFLICT (scope_key) DO UPDATE SET
    status = EXCLUDED.status,
    last_error = EXCLUDED.last_error,
    updated_at = now()
RETURNING *;
