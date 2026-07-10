# Development setup

This guide gets you ready to develop PatchBase locally.

## What you need

- **Go 1.25+**
- **Bazel** (via bazelisk)
- **Node.js 20+** and **pnpm**
- **Docker** and **Docker Compose** (for PostgreSQL and Mailpit)
- **golangci-lint** (for linting)

## 1. Start the dependencies

```bash
docker compose up -d
```

This starts:
- PostgreSQL on port `5432` (database: `patchbase`, password: `postgres`)
- Mailpit on ports `1025` (SMTP) and `8025` (web UI for email testing)

## 2. Create a config file

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` and set the encryption key and JWT secret:

```yaml
encryption_key: "0123456789abcdef0123456789abcdef"
api:
  jwt_secret_key: "0123456789abcdef0123456789abcdef"
database:
  url: "postgres://postgres:postgres@localhost:5432/patchbase?sslmode=disable"
advisory_sync:
  storage_dir: "/tmp/patchbase-dev"
```

For local development, the storage dir can be anywhere convenient.

## 3. Run migrations

```bash
bazel run //cmd/patchbase-server -- migrate
```

## 4. Start the server

```bash
bazel run //cmd/patchbase-server -- serve
```

The server is now running at `http://localhost:5199`.

## 5. Frontend development

The dashboard is a SvelteKit app in `dashboard/`. During development, you can run it with hot module replacement:

```bash
cd dashboard
pnpm install
pnpm dev
```

This starts a Vite dev server (typically on port 5173) that proxies API calls to the Go backend on port 5199.

:::note
When running the dashboard separately, the Go server still serves the built frontend at port 5199. The Vite dev server is only needed when you're actively working on the frontend. The built dashboard is embedded into the Go binary at build time.
:::

## 6. Test database

For integration tests, there's a separate Docker Compose file (`compose.test.yaml`) that runs PostgreSQL on port `5433`:

```bash
docker compose -f compose.test.yaml up -d
```

If you change the database schema, recreate the test database volume so it loads the updated schema:

```bash
docker compose -f compose.test.yaml down -v
docker compose -f compose.test.yaml up -d
```

Or apply migrations manually:

```bash
bazel run //cmd/patchbase-server -- migrate --database-url "postgres://postgres:postgres@localhost:5433/patchbase_test?sslmode=disable"
```

## Useful tips

- Add `--automigrate` to the serve command to run migrations before each start (handy during development):
  ```bash
  bazel run //cmd/patchbase-server -- serve --automigrate
  ```
- The `config.yaml` in the repo root is git-ignored, so you can keep your local config there.
- Mailpit's web UI at `http://localhost:8025` shows all emails the server "sends."