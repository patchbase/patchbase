# Regenerating sqlc bindings

PatchBase uses [sqlc](https://github.com/sqlc-dev/sqlc) to generate type-safe Go code from SQL queries. The configuration lives at `db/sqlc.yaml`, queries are in `db/queries/`, and generated code goes to `internal/sql/`.

## The workflow

1. **Write or modify SQL queries** in `db/queries/*.sql`
2. **Regenerate schema** — after migration changes, run:
   ```bash
   bazel run //db:regen_schema
   ```
   This spins up a temporary PostgreSQL container, applies migrations, updates `db/schema.sql`, and runs sqlc to regenerate Go bindings.

3. **Update BUILD.bazel** — if sqlc generated new files, run Gazelle:
   ```bash
   bazel run //:gazelle
   ```

## Writing queries

sqlc queries live in `db/queries/`. Example:

```sql
-- name: GetHostByID :one
SELECT * FROM hosts
WHERE id = $1;

-- name: ListHosts :many
SELECT * FROM hosts
ORDER BY created_at DESC;

-- name: InsertHost :exec
INSERT INTO hosts (id, display_name, hostname)
VALUES ($1, $2, $3);
```

Each query has a name and a command type:

| Type | Description |
|------|-------------|
| `:one` | Returns a single row (or error) |
| `:many` | Returns multiple rows |
| `:exec` | Executes without returning rows |
| `:execrows` | Returns affected row count |
| `:copyfrom` | Bulk insert |

## Generated code

After regeneration, `internal/sql/` contains:

- `queries.go` — Go functions for each query
- `models.go` — Go structs for each table
- `db.go` — Querier interface and Queries struct

The generated code uses `pgx` types (`pgtype.Text`, `pgtype.Timestamptz`, etc.) and the `utils.Option[T]` type for nullable columns.

## Using generated queries

```go
queries := sql.New(pool)
host, err := queries.GetHostByID(ctx, "h_xxx")
```

For optional results (return `ErrNotFound` if no row):

```go
host, err := sql.Required(queries.GetHostByID(ctx, hostID))(apperr.ErrHostNotFound)
```