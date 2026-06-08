-- +goose Up

INSERT INTO settings (key, value) VALUES ('ask_to_copy_public_key', 'true'::jsonb) ON CONFLICT (key) DO NOTHING;
