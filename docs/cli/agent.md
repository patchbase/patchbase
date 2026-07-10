# `patchbase-agent` CLI

The PatchBase agent binary provides three subcommands.

## `patchbase-agent enroll`

Registers the host with a PatchBase server and writes the config file to disk. Run this once per host.

```bash
patchbase-agent enroll <server-url> <token> [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `server-url` | The PatchBase server URL (e.g., `https://patchbase.example.com:5199`) |
| `token` | A registration token created from the dashboard (e.g., `pb_reg_xxxxxx`) |

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--config` | `-c` | string | `/etc/patchbase-agent/config.json` | Config file path to write |
| `--ca-cert` | | string | *(empty)* | Path to CA certificate for verifying the server |
| `--allow-insecure-http` | `-k` | bool | `false` | Allow plain HTTP (no TLS) |

### Examples

```bash
# Basic enrollment
patchbase-agent enroll https://patchbase.example.com:5199 pb_reg_abc123

# With self-signed certificate
patchbase-agent enroll https://patchbase.example.com:5199 pb_reg_abc123 \
  --ca-cert /etc/patchbase-agent/ca.pem

# Development with plain HTTP
patchbase-agent enroll http://localhost:5199 pb_reg_abc123 -k

# Write config to a custom location
patchbase-agent enroll https://server:5199 pb_reg_abc123 -c /tmp/agent-config.json
```

## `patchbase-agent sync`

Collects a system snapshot and sends it to the PatchBase server.

```bash
patchbase-agent sync [flags]
```

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--config` | `-c` | string | `/etc/patchbase-agent/config.json` | Config file path |
| `--server-url` | `-s` | string | *(from config)* | Override server URL |
| `--token` | `-t` | string | *(from config)* | Override host token |
| `--ca-cert` | | string | *(from config)* | CA certificate path |
| `--allow-insecure-http` | `-k` | bool | `false` | Allow plain HTTP |
| `--debug` | | bool | `false` | Print snapshot JSON to stdout instead of sending |

### Examples

```bash
# Normal sync (reads config from /etc/patchbase-agent/config.json)
patchbase-agent sync

# Sync with debug output
patchbase-agent sync --debug

# Sync without a config file
patchbase-agent sync --server-url https://server:5199 --token pb_host_xxx
```

### What happens on failure

If the server rejects the snapshot (e.g., host not approved, token revoked), the agent prints the error code and message. The exit code is non-zero. Check the host approval status in the dashboard if you see rejection errors.

## `patchbase-agent version`

Prints the agent version and exits.

```bash
patchbase-agent version
```