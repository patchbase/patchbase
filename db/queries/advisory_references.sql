-- name: InsertAdvisoryReference :exec
INSERT INTO advisory_references (
    id, advisory_id, ref_type, ref_value, severity_vendor, severity_cvss, title, url
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (id) DO UPDATE SET
    advisory_id = EXCLUDED.advisory_id,
    ref_type = EXCLUDED.ref_type,
    ref_value = EXCLUDED.ref_value,
    severity_vendor = EXCLUDED.severity_vendor,
    severity_cvss = EXCLUDED.severity_cvss,
    title = EXCLUDED.title,
    url = EXCLUDED.url;

-- name: DeleteAdvisoryReferencesByStreamIDs :exec
DELETE FROM advisory_references
WHERE advisory_id IN (
    SELECT DISTINCT advisory_id FROM advisory_product_streams
    WHERE product_stream_id = ANY($1::text[])
);

-- name: ListAdvisoryReferencesByStreamIDs :many
SELECT ar.* FROM advisory_references ar
JOIN advisory_product_streams aps ON aps.advisory_id = ar.advisory_id
WHERE aps.product_stream_id = ANY($1::text[]);
