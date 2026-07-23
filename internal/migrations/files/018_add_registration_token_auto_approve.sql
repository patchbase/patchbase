-- +goose Up
ALTER TABLE registration_tokens ADD COLUMN auto_approve boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE registration_tokens DROP COLUMN auto_approve;
