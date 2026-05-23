-- name: InsertAdvisoryProductStream :exec
INSERT INTO advisory_product_streams (
    advisory_id, product_stream_id
) VALUES (
    $1, $2
)
ON CONFLICT (advisory_id, product_stream_id) DO NOTHING;

-- name: ListAdvisoryProductStreamsByStreamIDs :many
SELECT * FROM advisory_product_streams
WHERE product_stream_id = ANY($1::text[]);

-- name: DeleteAdvisoryProductStreamsByStreamIDs :exec
DELETE FROM advisory_product_streams
WHERE product_stream_id = ANY($1::text[]);
