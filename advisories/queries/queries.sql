-- name: ListProductStreams :many
SELECT *
FROM product_streams
ORDER BY vendor, distro_name, major_version, minor_version, architecture;

-- name: ListAdvisoriesByProductStream :many
SELECT a.*
FROM advisories AS a
JOIN advisory_product_streams AS aps ON aps.advisory_id = a.id
WHERE aps.product_stream_id = ?
ORDER BY a.updated_at DESC, a.published_at DESC, a.id;

-- name: ListAdvisoryReferences :many
SELECT *
FROM advisory_references
WHERE advisory_id = ?
ORDER BY ref_type, ref_value;

-- name: ListFixedPackagesByProductStream :many
SELECT *
FROM fixed_packages
WHERE product_stream_id = ?
ORDER BY package_name, epoch, version, release, arch;
