-- +goose Up

CREATE TABLE audit_log (
    id text PRIMARY KEY,
    actor_id text,
    actor_email text NOT NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text,
    metadata jsonb,
    ip_address text,
    user_agent text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT audit_log_id_prefix_check CHECK (id LIKE 'audit\_%' ESCAPE '\')
);

CREATE INDEX audit_log_created_at_idx ON audit_log (created_at DESC);
CREATE INDEX audit_log_actor_id_idx ON audit_log (actor_id);
CREATE INDEX audit_log_action_idx ON audit_log (action);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id);

-- +goose Down

DROP TABLE IF EXISTS audit_log;
