-- +goose Up

ALTER TABLE hosts
ADD COLUMN notes text;

-- +goose Down

ALTER TABLE hosts
DROP COLUMN IF EXISTS notes;
