# Quick start with Docker Compose

This guide gets the PatchBase server running on your machine in a few minutes using Docker Compose. It's the fastest way to try things out.

## 1. Clone the repository

```bash
git clone https://github.com/patchbase/patchbase.git
cd patchbase
```

## 2. Start PostgreSQL

The repository includes a `compose.yaml` that runs PostgreSQL and Mailpit (for email testing):

```bash
docker compose up -d
```

This starts PostgreSQL on port `5432` with a database called `patchbase` (password `postgres`).

## 3. Create a configuration file

Copy the example config and adjust the secrets:

```bash
cp config.example.yaml config.yaml
```

Open `config.yaml` and replace the placeholder values:

```yaml
encryption_key: "<run: openssl rand -hex 32>"

api:
  jwt_secret_key: "<run: openssl rand -hex 32>"

database:
  url: "postgres://postgres:postgres@localhost:5432/patchbase?sslmode=disable"
```

:::caution
The `encryption_key` is used to encrypt SSH credentials stored in the database. **Generate a strong key** and keep it safe — if you lose it, encrypted SSH keys become unrecoverable.
:::

## 4. Run database migrations

Before starting the server, apply the database schema:

```bash
patchbase-server migrate
```

If you built from source with Bazel, the binary is at `bazel-bin/cmd/patchbase-server/patchbase-server`. You can also run it directly:

```bash
bazel run //cmd/patchbase-server -- migrate
```

Migrations are idempotent — running them again is a no-op if everything is up to date.

## 5. Start the server

```bash
patchbase-server serve
```

Or with Bazel:

```bash
bazel run //cmd/patchbase-server -- serve
```

The server starts on `http://localhost:5199`. Open it in your browser and you'll see the setup wizard, which will guide you through creating your admin account.

## 6. Onboard your first host

Once the setup wizard is complete, head to the [onboarding guide](../onboarding/overview) to start tracking your first host.

## Next steps

- [Install from RPM packages](./rpm-packages) for a production deployment
- [Configure SSL/TLS](../guides/ssl-tls) for secure access
- [Set up advisory database sync](../guides/advisory-sync) to start matching vulnerabilities