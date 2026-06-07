-- name: CreateAdminUser :one
INSERT INTO users (
    id,
    email,
    name,
    password_hash,
    is_admin,
    password_reset_required
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    TRUE,
    TRUE
)
RETURNING *;

-- name: GetAdminUser :one
SELECT *
FROM users
WHERE is_admin = TRUE
  AND archived_at IS NULL
ORDER BY created_at ASC
LIMIT 1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1
  AND archived_at IS NULL
LIMIT 1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1
  AND archived_at IS NULL
LIMIT 1;

-- name: CompleteInitialSetupForUser :one
UPDATE users
SET
    email = $2,
    name = $3,
    password_hash = $4,
    password_reset_required = FALSE
WHERE id = $1
  AND archived_at IS NULL
RETURNING *;

-- name: UpdateUserEmail :one
UPDATE users
SET email = $2
WHERE id = $1
  AND archived_at IS NULL
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET
    password_hash = $2,
    password_reset_required = FALSE
WHERE id = $1
  AND archived_at IS NULL
RETURNING *;
