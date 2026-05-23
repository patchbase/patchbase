-- name: UpsertProductStream :exec
INSERT INTO product_streams (
    id, vendor, distro_family, distro_name, major_version, minor_version, architecture, repo_family, repo_id_pattern, cpe, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
ON CONFLICT (id) DO UPDATE SET
    vendor = EXCLUDED.vendor,
    distro_family = EXCLUDED.distro_family,
    distro_name = EXCLUDED.distro_name,
    major_version = EXCLUDED.major_version,
    minor_version = EXCLUDED.minor_version,
    architecture = EXCLUDED.architecture,
    repo_family = EXCLUDED.repo_family,
    repo_id_pattern = EXCLUDED.repo_id_pattern,
    cpe = EXCLUDED.cpe,
    status = EXCLUDED.status;

-- name: ListProductStreams :many
SELECT * FROM product_streams;

-- name: DeleteProductStreamsByIDs :exec
DELETE FROM product_streams
WHERE id = ANY($1::text[]);

-- name: ListProductStreamIDsByVendorAndVersion :many
SELECT id FROM product_streams
WHERE vendor = $1 AND major_version = $2;
