# `patchbase-server` CLI

The PatchBase server binary provides three subcommands.

## `patchbase-server serve`

Starts the HTTP server (API + dashboard).

```bash
patchbase-server serve [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--automigrate` | bool | `false` | Run database migrations before starting the server |

### Examples

```bash
# Start the server
patchbase-server serve

# Start with automatic migrations
patchbase-server serve --automigrate
```

The server reads `config.yaml` from the current directory (or `/etc/patchbase-server/config.yaml`).

## `patchbase-server migrate`

Applies pending database migrations. Safe to run repeatedly — it's a no-op if the database is up to date.

```bash
patchbase-server migrate [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--database-url` | string | *(from config)* | Override the database URL from config |

### Examples

```bash
# Migrate using config.yaml
patchbase-server migrate

# Migrate a specific database
patchbase-server migrate --database-url "postgres://user:pass@host:5432/patchbase?sslmode=disable"
```

## `patchbase-server version`

Prints the server version and exits.

```bash
patchbase-server version
```