-- name: InsertAffectedPackageRule :exec
INSERT INTO affected_package_rules (
    id, advisory_id, product_stream_id, package_name, source_rpm, arch, epoch_constraint, version_constraint, release_constraint, rpm_evr_rule, context, evidence_tier
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (id) DO UPDATE SET
    advisory_id = EXCLUDED.advisory_id,
    product_stream_id = EXCLUDED.product_stream_id,
    package_name = EXCLUDED.package_name,
    source_rpm = EXCLUDED.source_rpm,
    arch = EXCLUDED.arch,
    epoch_constraint = EXCLUDED.epoch_constraint,
    version_constraint = EXCLUDED.version_constraint,
    release_constraint = EXCLUDED.release_constraint,
    rpm_evr_rule = EXCLUDED.rpm_evr_rule,
    context = EXCLUDED.context,
    evidence_tier = EXCLUDED.evidence_tier;

-- name: ListAffectedPackageRulesByStreamIDs :many
SELECT * FROM affected_package_rules
WHERE product_stream_id = ANY($1::text[]);

-- name: DeleteAffectedPackageRulesByStreamIDs :exec
DELETE FROM affected_package_rules
WHERE product_stream_id = ANY($1::text[]);
