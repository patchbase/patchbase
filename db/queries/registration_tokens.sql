-- name: InsertRegistrationToken :one
INSERT INTO registration_tokens (
    id,
    name,
    token_hash,
    created_by_user_id
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: ListRegistrationTokens :many
SELECT *
FROM registration_tokens
ORDER BY created_at DESC, id DESC;

-- name: GetActiveRegistrationTokenByHash :one
SELECT *
FROM registration_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
LIMIT 1;

-- name: RevokeRegistrationToken :one
UPDATE registration_tokens
SET revoked_at = now()
WHERE id = $1
  AND revoked_at IS NULL
RETURNING *;

-- name: TouchRegistrationTokenLastUsed :exec
UPDATE registration_tokens
SET last_used_at = now()
WHERE id = $1;
