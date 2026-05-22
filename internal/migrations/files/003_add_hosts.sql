-- +goose Up

CREATE TABLE hosts (
    id text PRIMARY KEY,
    onboarding_mode text NOT NULL,
    approval_status text NOT NULL DEFAULT 'waiting_approval',
    approved_at timestamptz,
    display_name text,
    machine_id text,
    hostname text,
    ip_address text,
    os_family text NOT NULL DEFAULT 'unknown',
    os_name text NOT NULL DEFAULT 'Unknown',
    os_major integer NOT NULL DEFAULT 0,
    os_version text NOT NULL DEFAULT 'unknown',
    architecture text NOT NULL DEFAULT 'unknown',
    status text NOT NULL DEFAULT 'active',
    last_seen_at timestamptz,
    last_advisory_check_at timestamptz,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_snapshot_id text,
    pull_ssh_user text,
    pull_frequency_minutes integer,
    pull_public_key text,
    pull_private_key text,
    pull_last_run_at timestamptz,
    pull_last_run_status text,
    pull_last_run_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT hosts_id_prefix_check CHECK (id LIKE 'h\_%' ESCAPE '\'),
    CONSTRAINT hosts_onboarding_mode_check CHECK (onboarding_mode IN ('agent', 'ssh')),
    CONSTRAINT hosts_approval_status_check CHECK (approval_status IN ('waiting_approval', 'approved', 'rejected'))
);

CREATE INDEX hosts_approval_status_idx ON hosts (approval_status);
CREATE INDEX hosts_hostname_idx ON hosts (hostname);

CREATE TRIGGER hosts_set_updated_at
BEFORE UPDATE ON hosts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE registration_tokens (
    id text PRIMARY KEY,
    name text NOT NULL,
    token_hash text NOT NULL,
    created_by_user_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    last_used_at timestamptz,
    CONSTRAINT registration_tokens_id_prefix_check CHECK (id LIKE 'rtok\_%' ESCAPE '\'),
    CONSTRAINT registration_tokens_created_by_user_fk FOREIGN KEY (created_by_user_id) REFERENCES users (id)
);

CREATE UNIQUE INDEX registration_tokens_token_hash_unique_idx ON registration_tokens (token_hash);
CREATE INDEX registration_tokens_active_idx ON registration_tokens (created_at DESC) WHERE revoked_at IS NULL;

CREATE TABLE host_access_tokens (
    id text PRIMARY KEY,
    host_id text NOT NULL,
    token_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    last_used_at timestamptz,
    CONSTRAINT host_access_tokens_id_prefix_check CHECK (id LIKE 'htok\_%' ESCAPE '\'),
    CONSTRAINT host_access_tokens_host_fk FOREIGN KEY (host_id) REFERENCES hosts (id)
);

CREATE UNIQUE INDEX host_access_tokens_token_hash_unique_idx ON host_access_tokens (token_hash);
CREATE INDEX host_access_tokens_host_idx ON host_access_tokens (host_id);
CREATE INDEX host_access_tokens_active_idx ON host_access_tokens (host_id, created_at DESC) WHERE revoked_at IS NULL;

CREATE TABLE host_snapshots (
    id text PRIMARY KEY,
    host_id text NOT NULL,
    collected_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    payload bytea NOT NULL,
    running_kernel_nevra text NOT NULL DEFAULT '',
    boot_time timestamptz,
    has_process_data boolean NOT NULL DEFAULT FALSE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT host_snapshots_id_prefix_check CHECK (id LIKE 'snap\_%' ESCAPE '\'),
    CONSTRAINT host_snapshots_host_fk FOREIGN KEY (host_id) REFERENCES hosts (id)
);

CREATE INDEX host_snapshots_host_collected_idx ON host_snapshots (host_id, collected_at DESC);

ALTER TABLE hosts
ADD CONSTRAINT hosts_last_snapshot_fk
FOREIGN KEY (last_snapshot_id) REFERENCES host_snapshots (id);

CREATE TABLE host_current_state (
    host_id text PRIMARY KEY,
    snapshot_id text NOT NULL,
    overall_action text NOT NULL DEFAULT 'none',
    critical_count integer NOT NULL DEFAULT 0,
    important_count integer NOT NULL DEFAULT 0,
    moderate_count integer NOT NULL DEFAULT 0,
    actionable_count integer NOT NULL DEFAULT 0,
    available_updates integer NOT NULL DEFAULT 0,
    needs_reboot integer NOT NULL DEFAULT 0,
    needs_restart integer NOT NULL DEFAULT 0,
    no_fix integer NOT NULL DEFAULT 0,
    unknown integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT host_current_state_host_fk FOREIGN KEY (host_id) REFERENCES hosts (id),
    CONSTRAINT host_current_state_snapshot_fk FOREIGN KEY (snapshot_id) REFERENCES host_snapshots (id)
);
