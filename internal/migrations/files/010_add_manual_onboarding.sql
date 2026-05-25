-- +goose Up
ALTER TABLE hosts DROP CONSTRAINT hosts_onboarding_mode_check;
ALTER TABLE hosts ADD CONSTRAINT hosts_onboarding_mode_check CHECK (onboarding_mode IN ('agent', 'ssh', 'manual'));

-- +goose Down
ALTER TABLE hosts DROP CONSTRAINT hosts_onboarding_mode_check;
ALTER TABLE hosts ADD CONSTRAINT hosts_onboarding_mode_check CHECK (onboarding_mode IN ('agent', 'ssh'));
