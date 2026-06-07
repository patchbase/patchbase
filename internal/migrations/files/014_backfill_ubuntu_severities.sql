-- +goose Up
-- +goose StatementBegin
WITH decision_severities AS (
  SELECT
    dr.snapshot_id,
    dr.id,
    COALESCE(
      CASE lower(NULLIF(trim(dr.severity), ''))
        WHEN 'critical' THEN 4
        WHEN 'important' THEN 3
        WHEN 'high' THEN 3
        WHEN 'moderate' THEN 2
        WHEN 'medium' THEN 2
        WHEN 'low' THEN 1
      END,
      CASE lower(NULLIF(trim(a.severity), ''))
        WHEN 'critical' THEN 4
        WHEN 'important' THEN 3
        WHEN 'high' THEN 3
        WHEN 'moderate' THEN 2
        WHEN 'medium' THEN 2
        WHEN 'low' THEN 1
      END,
      MAX(
        CASE lower(NULLIF(trim(ar.severity_vendor), ''))
          WHEN 'critical' THEN 4
          WHEN 'important' THEN 3
          WHEN 'high' THEN 3
          WHEN 'moderate' THEN 2
          WHEN 'medium' THEN 2
          WHEN 'low' THEN 1
        END
      )
    ) AS severity_priority
  FROM decision_records dr
  LEFT JOIN advisories a ON a.id = dr.advisory_id
  LEFT JOIN advisory_references ar ON ar.advisory_id = dr.advisory_id
  WHERE dr.status <> 'resolved'
  GROUP BY dr.snapshot_id, dr.id, dr.severity, a.severity
)
UPDATE host_current_state hcs
SET
  critical_count = (
    SELECT COUNT(*)
    FROM decision_severities ds
    WHERE ds.snapshot_id = hcs.snapshot_id
      AND ds.severity_priority = 4
  ),
  important_count = (
    SELECT COUNT(*)
    FROM decision_severities ds
    WHERE ds.snapshot_id = hcs.snapshot_id
      AND ds.severity_priority = 3
  ),
  moderate_count = (
    SELECT COUNT(*)
    FROM decision_severities ds
    WHERE ds.snapshot_id = hcs.snapshot_id
      AND ds.severity_priority = 2
  )
WHERE EXISTS (
  SELECT 1
  FROM decision_records dr2
  WHERE dr2.snapshot_id = hcs.snapshot_id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Down migration is intentionally empty because the up migration just corrects data consistency.
-- +goose StatementEnd
