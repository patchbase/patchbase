-- +goose Up
ALTER TABLE host_ssh_pull ADD COLUMN pull_hostname text;

UPDATE host_ssh_pull hp
SET pull_hostname = h.hostname
FROM hosts h
WHERE h.id = hp.host_id;

ALTER TABLE host_ssh_pull ALTER COLUMN pull_hostname SET NOT NULL;

-- +goose Down
ALTER TABLE host_ssh_pull DROP COLUMN pull_hostname;
