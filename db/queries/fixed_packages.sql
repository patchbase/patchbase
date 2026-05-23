-- name: InsertFixedPackage :exec
INSERT INTO fixed_packages (
    id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, source_rpm, repo_family, evidence_tier
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (id) DO UPDATE SET
    advisory_id = EXCLUDED.advisory_id,
    product_stream_id = EXCLUDED.product_stream_id,
    package_name = EXCLUDED.package_name,
    epoch = EXCLUDED.epoch,
    version = EXCLUDED.version,
    release = EXCLUDED.release,
    arch = EXCLUDED.arch,
    nevra = EXCLUDED.nevra,
    source_rpm = EXCLUDED.source_rpm,
    repo_family = EXCLUDED.repo_family,
    evidence_tier = EXCLUDED.evidence_tier;

-- name: ListFixedPackagesByStreamIDs :many
SELECT * FROM fixed_packages
WHERE product_stream_id = ANY($1::text[]);

-- name: DeleteFixedPackagesByStreamIDs :exec
DELETE FROM fixed_packages
WHERE product_stream_id = ANY($1::text[]);
