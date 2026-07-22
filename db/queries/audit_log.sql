-- name: InsertAuditLog :exec
INSERT INTO audit_log (
    id,
    actor_id,
    actor_email,
    action,
    target_type,
    target_id,
    metadata,
    ip_address,
    user_agent
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: ListAuditLogs :many
SELECT id, actor_id, actor_email, action, target_type, target_id, metadata, ip_address, user_agent, created_at
FROM audit_log
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_log;

-- name: ListAuditLogsFiltered :many
SELECT id, actor_id, actor_email, action, target_type, target_id, metadata, ip_address, user_agent, created_at
FROM audit_log
WHERE
    (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action')::text)
    AND (sqlc.narg('actor_id')::text IS NULL OR actor_id = sqlc.narg('actor_id')::text)
    AND (sqlc.narg('from')::timestamptz IS NULL OR created_at >= sqlc.narg('from')::timestamptz)
    AND (sqlc.narg('to')::timestamptz IS NULL OR created_at <= sqlc.narg('to')::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAuditLogsFiltered :one
SELECT COUNT(*) FROM audit_log
WHERE
    (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action')::text)
    AND (sqlc.narg('actor_id')::text IS NULL OR actor_id = sqlc.narg('actor_id')::text)
    AND (sqlc.narg('from')::timestamptz IS NULL OR created_at >= sqlc.narg('from')::timestamptz)
    AND (sqlc.narg('to')::timestamptz IS NULL OR created_at <= sqlc.narg('to')::timestamptz);
