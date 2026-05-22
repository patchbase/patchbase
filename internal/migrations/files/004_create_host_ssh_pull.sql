-- +goose Up

CREATE TABLE host_ssh_pull (
    host_id text NOT NULL PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
    pull_ssh_user text,
    pull_frequency_minutes integer,
    pull_public_key text,
    pull_private_key text,
    pull_last_run_at timestamp with time zone,
    pull_last_run_status text,
    pull_last_run_error text
);

ALTER TABLE hosts
    DROP COLUMN pull_ssh_user,
    DROP COLUMN pull_frequency_minutes,
    DROP COLUMN pull_public_key,
    DROP COLUMN pull_private_key,
    DROP COLUMN pull_last_run_at,
    DROP COLUMN pull_last_run_status,
    DROP COLUMN pull_last_run_error;
