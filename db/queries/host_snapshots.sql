-- name: InsertHostSnapshot :one
INSERT INTO host_snapshots (
    id,
    host_id,
    collected_at,
    payload,
    running_kernel_nevra,
    boot_time,
    has_process_data
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;

-- name: GetLatestHostSnapshotByHostID :one
SELECT *
FROM host_snapshots
WHERE host_id = $1
ORDER BY collected_at DESC, id DESC
LIMIT 1;

-- name: DeleteHostCurrentStateByHostID :exec
DELETE FROM host_current_state
WHERE host_id = $1;

-- name: DeleteHostSnapshotsByHostID :exec
DELETE FROM host_snapshots
WHERE host_id = $1;

-- name: UpsertHostCurrentState :exec
INSERT INTO host_current_state (
    host_id,
    snapshot_id,
    overall_action,
    critical_count,
    important_count,
    moderate_count,
    actionable_count,
    available_updates,
    needs_reboot,
    needs_restart,
    no_fix,
    unknown,
    updated_at
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    now()
)
ON CONFLICT (host_id) DO UPDATE SET
    snapshot_id = EXCLUDED.snapshot_id,
    overall_action = EXCLUDED.overall_action,
    critical_count = EXCLUDED.critical_count,
    important_count = EXCLUDED.important_count,
    moderate_count = EXCLUDED.moderate_count,
    actionable_count = EXCLUDED.actionable_count,
    available_updates = EXCLUDED.available_updates,
    needs_reboot = EXCLUDED.needs_reboot,
    needs_restart = EXCLUDED.needs_restart,
    no_fix = EXCLUDED.no_fix,
    unknown = EXCLUDED.unknown,
    updated_at = now();

-- name: GetHostSnapshot :one
SELECT * FROM host_snapshots
WHERE id = $1
LIMIT 1;
