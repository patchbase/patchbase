# Server configuration

The PatchBase server reads its configuration from a YAML file. By default it looks for `config.yaml` in the current directory, but you can also place it at `/etc/patchbase-server/config.yaml`.

The example configuration is at [`config.example.yaml`](https://github.com/patchbase/patchbase/blob/main/config.example.yaml) in the repository.

## Full reference

```yaml
# Encryption key for SSH credentials stored in the database.
# Generate with: openssl rand -hex 32
# This must be set to a strong 32-character key.
encryption_key: "your-encryption-key"

# API configuration
api:
  # JWT secret key for signing session tokens.
  # Generate with: openssl rand -hex 32
  jwt_secret_key: "your-jwt-secret"
  
  # Address and port where the API server listens.
  # listen_address: "0.0.0.0"
  # port: 5199
  
  # Log level for HTTP requests: debug, info, warn, error
  # request_log_level: "debug"
  
  # Timeout settings
  # read_timeout: 5s
  # read_header_timeout: 5s
  # write_timeout: 60s
  # shutdown_timeout: 10s
  
  # Maximum size of inbound request bodies in bytes.
  # Requests larger than this are rejected with HTTP 413.
  # max_request_body_bytes: 33554432

# SSL / TLS configuration
ssl:
  # enabled: false
  # certificate_file: "/etc/patchbase-server/cert.pem"
  # key_file: "/etc/patchbase-server/key.pem"

# Database configuration
database:
  # PostgreSQL connection URL
  url: "postgres://postgres:postgres@localhost:5432/patchbase?sslmode=disable"
  
  # Query logging level: trace, debug, info, warn, error
  # log_level: "error"

# SSH pull configuration
ssh:
  # Maximum runtime for one SSH pull job, including connection,
  # collection, parsing, and database updates.
  # pull_job_timeout: 5m

# Advisory database synchronizer
advisory_sync:
  # Base URL where advisory database manifests and files are published.
  # base_url: "https://dl.patchbase.net/v1/advisory-db"
  
  # Interval between synchronization checks
  # refresh_interval: 6h
  
  # Local directory where downloaded SQLite databases are stored
  # storage_dir: "/var/lib/patchbase-server/db/advisories"
  
  # Custom scope mappings (optional, overrides defaults for matched hosts)
  # scope_mappings:
  #   - scope: "rocky:9"
  #     match:
  #       os_family: "rocky"
  #       os_major: 9
```

## Configuration sections

### `encryption_key`

**Required.** Used to encrypt SSH credentials stored in the database. Generate a strong key:

```bash
openssl rand -hex 32
```

:::warning
If you change this key after hosts are already configured with SSH pull mode, the existing encrypted SSH keys become unrecoverable. Keep this key safe and stable.
:::

### `api`

| Field | Default | Description |
|-------|---------|-------------|
| `jwt_secret_key` | *(required)* | Secret used to sign JWT session tokens |
| `listen_address` | `0.0.0.0` | Bind address |
| `port` | `5199` | Listen port |
| `request_log_level` | `debug` | HTTP request log verbosity |
| `read_timeout` | `5s` | HTTP read timeout |
| `read_header_timeout` | `5s` | HTTP header read timeout |
| `write_timeout` | `60s` | HTTP write timeout |
| `shutdown_timeout` | `10s` | Graceful shutdown deadline |
| `max_request_body_bytes` | `33554432` (32 MiB) | Max request body size |

### `ssl`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Whether HTTPS is enabled |
| `certificate_file` | `/etc/patchbase-server/cert.pem` | Path to TLS certificate |
| `key_file` | `/etc/patchbase-server/key.pem` | Path to TLS private key |

When `enabled` is `true`, both `certificate_file` and `key_file` must exist and be valid files.

### `database`

| Field | Default | Description |
|-------|---------|-------------|
| `url` | *(required)* | PostgreSQL connection URL |
| `log_level` | `error` | SQL query log verbosity |

The URL follows the standard PostgreSQL format: `postgres://user:password@host:port/dbname?sslmode=disable`

### `ssh`

| Field | Default | Description |
|-------|---------|-------------|
| `pull_job_timeout` | `5m` | Max runtime for a single SSH pull job |

### `advisory_sync`

| Field | Default | Description |
|-------|---------|-------------|
| `base_url` | `https://dl.patchbase.net/v1/advisory-db` | Advisory database manifest URL |
| `refresh_interval` | `6h` | How often to check for advisory updates |
| `storage_dir` | `/var/lib/patchbase-server/db/advisories` | Where to store downloaded SQLite databases |
| `scope_mappings` | *(built-in defaults)* | Custom host-to-scope mappings |

See [custom scope mappings](../guides/scope-mappings) for details on `scope_mappings`.