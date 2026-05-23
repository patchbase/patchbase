-- name: InsertAgentHost :one
INSERT INTO hosts (
    id,
    onboarding_mode,
    approval_status,
    display_name,
    machine_id,
    hostname,
    ip_address,
    os_name,
    os_version,
    architecture,
    status
)
VALUES (
    $1,
    'agent',
    'waiting_approval',
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    'active'
)
RETURNING *;

-- name: InsertSSHHost :one
WITH new_host AS (
    INSERT INTO hosts (
        id,
        onboarding_mode,
        approval_status,
        approved_at,
        display_name,
        hostname,
        ip_address,
        status
    )
    VALUES (
        $1,
        'ssh',
        'approved',
        now(),
        $2,
        $3,
        $4,
        'active'
    )
    RETURNING *
),
new_ssh_pull AS (
    INSERT INTO host_ssh_pull (
        host_id,
        pull_ssh_user,
        pull_frequency_minutes,
        pull_public_key,
        pull_private_key
    )
    VALUES (
        $1,
        $5,
        $6,
        $7,
        $8
    )
    RETURNING *
)
SELECT
    sqlc.embed(hosts),
    nsp.pull_ssh_user,
    nsp.pull_frequency_minutes,
    nsp.pull_public_key,
    nsp.pull_private_key,
    nsp.pull_last_run_at,
    nsp.pull_last_run_status,
    nsp.pull_last_run_error
FROM new_host hosts
JOIN new_ssh_pull nsp ON nsp.host_id = hosts.id;

-- name: GetHostByID :one
SELECT *
FROM hosts
WHERE id = $1;

-- name: ListPendingHosts :many
SELECT *
FROM hosts
WHERE approval_status = 'waiting_approval'
ORDER BY created_at DESC, id DESC;

-- name: ApproveHostByID :one
UPDATE hosts
SET
    approval_status = 'approved',
    approved_at = now()
WHERE id = $1
RETURNING *;

-- name: ClearHostLastSnapshotByID :exec
UPDATE hosts
SET last_snapshot_id = NULL
WHERE id = $1;

-- name: DeleteHostByID :one
DELETE FROM hosts
WHERE id = $1
RETURNING *;

-- name: UpdateHostFromSnapshot :one
UPDATE hosts
SET
    machine_id = $2,
    hostname = $3,
    ip_address = $4,
    os_family = $5,
    os_name = $6,
    os_major = $7,
    os_version = $8,
    architecture = $9,
    status = 'active',
    last_seen_at = $10,
    last_advisory_check_at = $10,
    last_snapshot_id = $11
WHERE id = $1
RETURNING *;

-- name: UpdateSSHPullRun :exec
WITH updated_pull AS (
    UPDATE host_ssh_pull
    SET
        pull_last_run_at = @pull_last_run_at,
        pull_last_run_status = @pull_last_run_status,
        pull_last_run_error = @pull_last_run_error
    WHERE host_id = @id
)
UPDATE hosts
SET
    last_advisory_check_at = @pull_last_run_at,
    machine_id = COALESCE(@machine_id, machine_id),
    hostname = COALESCE(@hostname, hostname),
    ip_address = COALESCE(@ip_address, ip_address),
    os_family = CASE WHEN @os_family::text <> '' AND @os_family::text <> 'unknown' THEN @os_family ELSE os_family END,
    os_name = CASE WHEN @os_name::text <> '' AND @os_name::text <> 'Unknown' THEN @os_name ELSE os_name END,
    os_major = CASE WHEN @os_major::integer <> 0 THEN @os_major ELSE os_major END,
    os_version = CASE WHEN @os_version::text <> '' AND @os_version::text <> 'unknown' THEN @os_version ELSE os_version END,
    architecture = CASE WHEN @architecture::text <> '' AND @architecture::text <> 'unknown' THEN @architecture ELSE architecture END,
    last_seen_at = COALESCE(@pull_last_run_at, last_seen_at)
WHERE id = @id;

-- name: ListHostsWithState :many
SELECT
    sqlc.embed(h),
    hp.pull_last_run_at,
    hp.pull_last_run_status,
    hp.pull_last_run_error,
    COALESCE(hcs.overall_action, 'none') AS overall_action,
    COALESCE(hcs.critical_count, 0) AS critical_count,
    COALESCE(hcs.important_count, 0) AS important_count,
    COALESCE(hcs.moderate_count, 0) AS moderate_count,
    COALESCE(hcs.actionable_count, 0) AS actionable_count,
    COALESCE(hcs.available_updates, 0) AS available_updates,
    COALESCE(hcs.needs_reboot, 0) AS needs_reboot,
    COALESCE(hcs.needs_restart, 0) AS needs_restart,
    COALESCE(hcs.no_fix, 0) AS no_fix,
    COALESCE(hcs.unknown, 0) AS unknown,
    hcs.updated_at AS state_updated_at
FROM hosts h
LEFT JOIN host_ssh_pull hp ON hp.host_id = h.id
LEFT JOIN host_current_state hcs ON hcs.host_id = h.id
ORDER BY
    CASE COALESCE(hcs.overall_action, 'none')
        WHEN 'reboot_host' THEN 0
        WHEN 'restart_service' THEN 1
        WHEN 'update_package' THEN 2
        WHEN 'investigate' THEN 3
        ELSE 4
    END,
    h.last_seen_at DESC NULLS LAST,
    h.hostname ASC,
    h.id ASC;

-- name: GetHostWithStateByID :one
SELECT
    sqlc.embed(h),
    hp.pull_last_run_at,
    hp.pull_last_run_status,
    hp.pull_last_run_error,
    COALESCE(hcs.overall_action, 'none') AS overall_action,
    COALESCE(hcs.critical_count, 0) AS critical_count,
    COALESCE(hcs.important_count, 0) AS important_count,
    COALESCE(hcs.moderate_count, 0) AS moderate_count,
    COALESCE(hcs.actionable_count, 0) AS actionable_count,
    COALESCE(hcs.available_updates, 0) AS available_updates,
    COALESCE(hcs.needs_reboot, 0) AS needs_reboot,
    COALESCE(hcs.needs_restart, 0) AS needs_restart,
    COALESCE(hcs.no_fix, 0) AS no_fix,
    COALESCE(hcs.unknown, 0) AS unknown,
    hcs.updated_at AS state_updated_at
FROM hosts h
LEFT JOIN host_ssh_pull hp ON hp.host_id = h.id
LEFT JOIN host_current_state hcs ON hcs.host_id = h.id
WHERE h.id = $1;

-- name: GetSSHPullConfigByHostID :one
SELECT *
FROM host_ssh_pull
WHERE host_id = $1;

-- name: UpdateHostAdvisoryScopeKey :exec
UPDATE hosts
SET advisory_scope_key = $2
WHERE id = $1;

-- name: ListApprovedSSHHosts :many
SELECT
    h.id,
    hp.pull_frequency_minutes
FROM hosts h
JOIN host_ssh_pull hp ON hp.host_id = h.id
WHERE h.onboarding_mode = 'ssh' AND h.approval_status = 'approved' AND hp.onboarded = true;

-- name: ListHostsByAdvisoryScopeKey :many
SELECT * FROM hosts
WHERE advisory_scope_key = $1;

-- name: SetSSHPullOnboarded :exec
UPDATE host_ssh_pull
SET onboarded = $2
WHERE host_id = $1;
