-- +goose Up

-- +goose StatementBegin
CREATE UNIQUE INDEX hosts_display_name_unique_idx ON hosts (display_name);
CREATE UNIQUE INDEX host_ssh_pull_pull_hostname_unique_idx ON host_ssh_pull (pull_hostname);
-- +goose StatementEnd
