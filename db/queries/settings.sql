-- name: CreateSetting :one
INSERT INTO settings (
    key,
    value
)
VALUES (
    $1,
    $2
)
RETURNING
    key,
    value,
    created_at,
    updated_at;

-- name: CreateSettingIfAbsent :exec
INSERT INTO settings (
    key,
    value
)
VALUES (
    $1,
    $2
)
ON CONFLICT (key) DO NOTHING;

-- name: GetSetting :one
SELECT
    key,
    value,
    created_at,
    updated_at
FROM settings
WHERE key = $1;

-- name: UpsertSetting :one
INSERT INTO settings (
    key,
    value
)
VALUES (
    $1,
    $2
)
ON CONFLICT (key)
DO UPDATE SET
    value = EXCLUDED.value
RETURNING
    key,
    value,
    created_at,
    updated_at;
