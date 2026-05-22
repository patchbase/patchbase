-- +goose Up

CREATE TABLE advisory_scopes (
    scope_key text PRIMARY KEY,
    status text NOT NULL DEFAULT 'pending',
    last_sync_at timestamp with time zone,
    last_success_at timestamp with time zone,
    last_error text,
    advisory_count integer NOT NULL DEFAULT 0,
    sha256 text,
    size_bytes bigint NOT NULL DEFAULT 0,
    local_path text,
    next_refresh_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

ALTER TABLE hosts
ADD COLUMN advisory_scope_key text REFERENCES advisory_scopes(scope_key) ON DELETE SET NULL;

CREATE INDEX hosts_advisory_scope_key_idx ON hosts (advisory_scope_key);

CREATE TRIGGER advisory_scopes_set_updated_at
BEFORE UPDATE ON advisory_scopes
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
