# FAQ and troubleshooting

## General

### Can I use PatchBase without internet access?

The server needs internet access to download advisory databases from `dl.patchbase.net`. Hosts can be onboarded in [manual mode](./onboarding/manual-mode) if they're air-gapped — you download the collector script, run it on the host, and upload the report.

If the server itself has no internet, advisory sync will fail and you won't get vulnerability matching. You could host the advisory databases on an internal mirror and point `advisory_sync.base_url` to it.

### How many hosts can PatchBase handle?

PatchBase is designed for small-to-medium fleets. There's no hard limit, but the default configuration handles hundreds of hosts comfortably. The main bottleneck is PostgreSQL performance — ensure your database has enough RAM for connection pooling and query caching.

### Does PatchBase send data anywhere?

No. PatchBase is fully self-hosted. The only outbound connections are:

- The server fetching advisory databases from `dl.patchbase.net`
- The agent sending snapshots to your server

No telemetry, no phone-home, no third-party analytics.

## Agent

### The agent can't connect to the server

Check the obvious things first:

1. Is the server running? (`curl http://server:5199/api/v1/health`)
2. Is the port open on the server's firewall?
3. Is the server URL correct in the agent config? (`cat /etc/patchbase-agent/config.json`)

If you're using HTTPS with a self-signed certificate, make sure you passed `--ca-cert` during enrollment.

### Host is stuck in "pending" status

Hosts registered via the agent appear as pending until approved. Go to **Hosts → Pending** in the dashboard and click **Approve**. The agent will succeed on its next sync attempt.

### `patchbase-agent sync` returns an error code

Common error codes:

| Code | Meaning |
|------|---------|
| `host_not_approved` | The host hasn't been approved in the dashboard yet |
| `invalid_host_access_token` | The token is wrong or has been revoked |
| `host_identity_mismatch` | The machine ID doesn't match what was registered |

## SSH pull

### SSH pull job fails with "permission denied (publickey)"

The server's SSH public key isn't installed on the target host. Go to the host detail page, copy the public key, and add it to `~/.ssh/authorized_keys` for the SSH user.

### SSH pull job fails with connection timeout

Check that:

1. The hostname is reachable from the server (`ssh -v user@hostname`)
2. SSH port 22 is open on the target host's firewall
3. The SSH user exists on the target host

### The wrong OS is detected

The server reads `/etc/os-release` to detect the OS family. If your distribution's `ID` or `ID_LIKE` fields are unusual, detection might fail. Check the contents of `/etc/os-release` on the target host and file an issue if the values aren't recognized.

## Advisory sync

### Advisory scope status is "failed"

Check the error message in the scope list (Advisories → Scopes). Common causes:

- The server can't reach `dl.patchbase.net` (network/firewall issue)
- The manifest URL is wrong (check `advisory_sync.base_url` in config)
- Disk full (the advisory databases can be several MB each)

### No advisories are showing up for a host

This usually means the host's advisory scope hasn't been synced yet. When the first host for a scope is onboarded, the scope is marked as pending and a sync job is queued. Check **Advisories → Scopes** — if the scope is still pending or syncing, wait for it to finish. If it's failed, see the troubleshooting above.

### The host's OS isn't in the default scope mappings

If you're running a distribution that isn't covered by the [default mappings](./guides/scope-mappings), you can add a custom mapping in `config.yaml`. If the advisory database for your distribution doesn't exist on `dl.patchbase.net`, matching won't work — file an issue and we'll look into adding support.