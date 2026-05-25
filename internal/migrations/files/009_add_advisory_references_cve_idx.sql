-- +goose Up
CREATE INDEX idx_advisory_references_advisory_id_ref_type ON advisory_references (advisory_id, ref_type);

-- +goose Down
DROP INDEX idx_advisory_references_advisory_id_ref_type;
