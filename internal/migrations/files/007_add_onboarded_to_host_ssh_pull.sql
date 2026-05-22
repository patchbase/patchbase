-- +goose Up
ALTER TABLE host_ssh_pull ADD COLUMN onboarded boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE host_ssh_pull DROP COLUMN onboarded;
