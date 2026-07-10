# Working with protos

PatchBase uses [Protocol Buffers](https://protobuf.dev/) for the agent-server communication. Proto definitions live in `proto/agent/`.

## Structure

```
proto/
  agent/
    *.proto        # protobuf definitions
  BUILD.bazel
  defs.bzl
```

The generated Go code is imported as `go.patchbase.net/proto/agent` and used by both the server and the agent.

## Regenerating after proto changes

After modifying any `.proto` file:

```bash
bazel run //proto:update
```

This regenerates the Go bindings and updates the build files.

## Key messages

### `AgentSnapshot`

The main payload sent by the agent to the server. Contains:

- `schema_version` — snapshot format version
- `sent_at` — timestamp
- `host` — host metadata (hostname, machine ID, OS, architecture, kernel, uptime)
- `packages` — list of installed packages
- `upgradable_packages` — list of packages with available updates
- `repos` — enabled repositories
- `runtime` — kernel info and process data

### `RegisterHostRequest` / `RegisterHostResponse`

Used during agent enrollment. The request carries the registration token and host metadata. The response includes the host ID and host access token.

### `SyncResponse`

Returned by the server after snapshot ingestion. Includes whether the snapshot was accepted, the snapshot ID, and the next check-in interval in seconds.

## Adding a new field

1. Add the field to the appropriate `.proto` message
2. Run `bazel run //proto:update`
3. Update the agent collector to populate the field
4. Update the server's ingestion logic to handle it
5. Run `bazel run //:gazelle` to update BUILD files

:::note
If you rename a field, consider keeping backward compatibility by using a new field number rather than reusing an old one.