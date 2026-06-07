-- +goose Up

INSERT INTO settings (key, value) VALUES ('default_ssh_pull_user', '"root"'::jsonb) ON CONFLICT (key) DO NOTHING;
