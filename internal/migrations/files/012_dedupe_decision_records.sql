-- +goose Up
-- +goose StatementBegin

-- 1. Clean up existing duplicates by keeping the most recently computed record for each unique key
WITH duplicates AS (
    SELECT id,
           ROW_NUMBER() OVER(
               PARTITION BY snapshot_id, advisory_id, package_name, COALESCE(installed_nevra, '')
               ORDER BY computed_at DESC, id DESC
           ) as row_num
    FROM decision_records
)
DELETE FROM decision_records
WHERE id IN (SELECT id FROM duplicates WHERE row_num > 1);

-- 2. Add unique constraint to prevent future logical duplicates
CREATE UNIQUE INDEX decision_records_unique_idx ON decision_records (
    snapshot_id, 
    advisory_id, 
    package_name, 
    COALESCE(installed_nevra, '')
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS decision_records_unique_idx;
-- +goose StatementEnd
