-- +goose Up

CREATE TABLE users (
    id text PRIMARY KEY,
    email text NOT NULL,
    name text NOT NULL,
    password_hash text NOT NULL,
    is_admin boolean NOT NULL DEFAULT false,
    password_reset_required boolean NOT NULL DEFAULT false,
    last_login_at timestamptz,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_id_prefix_check CHECK (id LIKE 'u\_%' ESCAPE '\')
);

CREATE UNIQUE INDEX users_email_active_unique_idx
ON users (email)
WHERE archived_at IS NULL;

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
