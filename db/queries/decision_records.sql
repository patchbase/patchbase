-- name: InsertDecisionRecord :exec
INSERT INTO decision_records (
    id, host_id, snapshot_id, advisory_id, installed_package_id, product_stream_id, package_name, installed_nevra, fixed_nevra, status, action, severity, evidence_tier, reason_code, reason_text, computed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
);

-- name: DeleteDecisionRecordsBySnapshot :exec
DELETE FROM decision_records
WHERE snapshot_id = $1;

-- name: ListDecisionPageRowsBySnapshot :many
SELECT
    dr.*,
    a.summary AS advisory_summary,
    a.is_security AS advisory_is_security,
    a.source_url AS advisory_source_url,
    a.advisory_type AS advisory_type,
    a.source_system AS advisory_source_system,
    a.updated_at AS advisory_updated_at,
    COALESCE(
        (SELECT json_agg(json_build_object('id', ref_value, 'url', COALESCE(url, '')))
         FROM advisory_references
         WHERE advisory_id = dr.advisory_id AND ref_type = 'cve'),
        '[]'::json
    ) AS cves
FROM decision_records dr
JOIN advisories a ON a.id = dr.advisory_id
WHERE dr.snapshot_id = $1;
