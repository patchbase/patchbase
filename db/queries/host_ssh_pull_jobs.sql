-- name: InsertHostSSHPullJob :one
INSERT INTO host_ssh_pull_jobs (
    id,
    host_id,
    status,
    started_at
)
VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateHostSSHPullJob :one
UPDATE host_ssh_pull_jobs
SET
    status = $2,
    completed_at = $3,
    error = $4
WHERE id = $1
RETURNING *;

-- name: ListHostSSHPullJobsByHostID :many
SELECT *
FROM host_ssh_pull_jobs
WHERE host_id = $1
ORDER BY started_at DESC
LIMIT $2;
