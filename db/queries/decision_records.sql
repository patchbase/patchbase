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
    dr.id,
    dr.host_id,
    dr.snapshot_id,
    dr.advisory_id,
    dr.installed_package_id,
    dr.product_stream_id,
    dr.package_name,
    dr.installed_nevra,
    dr.fixed_nevra,
    dr.status,
    dr.action,
    COALESCE(
        NULLIF(dr.severity, ''),
        NULLIF(a.severity, ''),
        (
            SELECT CASE MAX(
                CASE
                    WHEN lower(ar.severity_vendor) = 'critical' THEN 4
                    WHEN lower(ar.severity_vendor) IN ('important', 'high') THEN 3
                    WHEN lower(ar.severity_vendor) IN ('moderate', 'medium') THEN 2
                    WHEN lower(ar.severity_vendor) = 'low' THEN 1
                    ELSE 0
                END
            )
                WHEN 4 THEN 'critical'
                WHEN 3 THEN 'important'
                WHEN 2 THEN 'moderate'
                WHEN 1 THEN 'low'
                ELSE NULL
            END
            FROM advisory_references ar
            WHERE ar.advisory_id = dr.advisory_id
        )
    ) AS severity,
    dr.evidence_tier,
    dr.reason_code,
    dr.reason_text,
    dr.computed_at,
    a.summary AS advisory_summary,
    a.is_security AS advisory_is_security,
    a.source_url AS advisory_source_url,
    a.advisory_type AS advisory_type,
    a.source_system AS advisory_source_system,
    a.updated_at AS advisory_updated_at,
    COALESCE(
        (SELECT json_agg(json_build_object('id', ref_value, 'url', COALESCE(url, ''), 'score', severity_cvss))
         FROM advisory_references
         WHERE advisory_id = dr.advisory_id AND ref_type = 'cve'),
        '[]'::json
    ) AS cves
FROM decision_records dr
JOIN advisories a ON a.id = dr.advisory_id
WHERE dr.snapshot_id = $1;
