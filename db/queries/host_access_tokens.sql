-- name: InsertHostAccessToken :one
INSERT INTO host_access_tokens (
    id,
    host_id,
    token_hash
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: GetActiveHostAccessTokenByHash :one
SELECT *
FROM host_access_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
LIMIT 1;

-- name: TouchHostAccessTokenLastUsed :exec
UPDATE host_access_tokens
SET last_used_at = now()
WHERE id = $1;

-- name: DeleteHostAccessTokensByHostID :exec
DELETE FROM host_access_tokens
WHERE host_id = $1;
