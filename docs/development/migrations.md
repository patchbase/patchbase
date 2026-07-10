# Database migrations

PatchBase uses [Goose](https://github.com/pressly/goose) for database migrations. Migration files are SQL files stored in `internal/migrations/files/`.

## Creating a migration

1. Create a new SQL file in `internal/migrations/files/` following the numbering convention:

```bash
internal/migrations/files/005_create_new_table.sql
```

2. Write the migration with up and down sections:

```sql
-- +goose Up
CREATE TABLE new_table (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE new_table;
```

3. Add the file to `embedsrcs` in `internal/migrations/BUILD.bazel`:

```python
go_library(
    name = "migrations",
    srcs = ["migrate.go"],
    embedsrcs = [
        "files/001_initial_schema.sql",
        "files/002_create_hosts.sql",
        "files/003_create_advisories.sql",
        "files/004_create_host_ssh_pull.sql",
        "files/005_create_new_table.sql",  # add your new file here
    ],
    ...
)
```

:::important
If you forget to add the file to `embedsrcs`, the migration won't be embedded in the binary and won't run.
:::

## Running migrations

```bash
# Using the CLI
bazel run //cmd/patchbase-server -- migrate

# With a custom database URL
bazel run //cmd/patchbase-server -- migrate --database-url "postgres://..."
```

Migrations are idempotent — running them when everything is up to date is a no-op.

## Regenerating schema and sqlc bindings

After adding or modifying a migration, regenerate the database schema and sqlc Go bindings:

```bash
bazel run //db:regen_schema
```

This:
1. Spins up a temporary PostgreSQL container
2. Applies all migrations
3. Updates `db/schema.sql`
4. Regenerates Go sqlc bindings under `internal/sql/`

## Updating the test database

After schema changes, recreate the test database volume:

```bash
docker compose -f compose.test.yaml down -v
docker compose -f compose.test.yaml up -d
```

Or apply migrations manually:

```bash
bazel run //cmd/patchbase-server -- migrate \
  --database-url "postgres://postgres:postgres@localhost:5433/patchbase_test?sslmode=disable"
```