-- +goose Up

CREATE TABLE host_ssh_pull_jobs (
    id text NOT NULL PRIMARY KEY,
    host_id text NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    status text NOT NULL,
    started_at timestamp with time zone NOT NULL DEFAULT now(),
    completed_at timestamp with time zone,
    error text
);
