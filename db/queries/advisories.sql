-- name: UpsertAdvisory :exec
INSERT INTO advisories (
    id, source_system, raw_source_id, source_url, vendor, advisory_type, severity, summary, description, published_at, updated_at, evidence_tier, is_security
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
ON CONFLICT (id) DO UPDATE SET
    source_system = EXCLUDED.source_system,
    raw_source_id = EXCLUDED.raw_source_id,
    source_url = EXCLUDED.source_url,
    vendor = EXCLUDED.vendor,
    advisory_type = EXCLUDED.advisory_type,
    severity = EXCLUDED.severity,
    summary = EXCLUDED.summary,
    description = EXCLUDED.description,
    published_at = EXCLUDED.published_at,
    updated_at = EXCLUDED.updated_at,
    evidence_tier = EXCLUDED.evidence_tier,
    is_security = EXCLUDED.is_security;

-- name: ListAdvisoriesByStreamIDs :many
SELECT DISTINCT a.* FROM advisories a
JOIN advisory_product_streams aps ON aps.advisory_id = a.id
WHERE aps.product_stream_id = ANY($1::text[]);

-- name: DeleteAdvisoriesWithoutStreams :exec
DELETE FROM advisories
WHERE id NOT IN (
    SELECT DISTINCT advisory_id FROM advisory_product_streams
);

-- name: ListRecentAdvisories :many
SELECT * FROM advisories
ORDER BY published_at DESC
LIMIT 5;

-- name: GetAdvisory :one
SELECT * FROM advisories
WHERE id = $1;
