# Agent configuration

The PatchBase agent stores its configuration in a JSON file at `/etc/patchbase-agent/config.json`. This file is created automatically during enrollment — you usually don't need to edit it by hand.

## File format

```json
{
  "server_url": "https://patchbase.example.com:5199",
  "host_token": "pb_host_xxxxxxxxxxxxxxxx",
  "ca_cert": "/etc/patchbase-agent/ca.pem",
  "allow_insecure_http": false
}
```

## Fields

| Field | Type | Description |
|-------|------|-------------|
| `server_url` | string | The PatchBase server URL (including scheme and port) |
| `host_token` | string | Host access token issued during enrollment |
| `ca_cert` | string | Path to a CA certificate file for verifying the server (optional) |
| `allow_insecure_http` | bool | Allow plain HTTP connections (for development only) |

## Overriding with flags

You can override config file values on the command line:

```bash
# Override server URL
patchbase-agent sync --server-url https://other-server:5199

# Override token
patchbase-agent sync --token pb_host_other

# Use a different config file
patchbase-agent sync --config /custom/path/config.json
```

Flags take precedence over the config file. If no config file exists, both `--server-url` and `--token` must be provided.

## File permissions

The config file is written with `0600` permissions (owner read/write only) because it contains the host access token. The directory `/etc/patchbase-agent/` is created with `0755` during enrollment.