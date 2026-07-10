# Build from source

PatchBase uses [Bazel](https://bazel.build) as its build system. You'll need Bazel 7+ (we recommend [bazelisk](https://github.com/bazelbuild/bazelisk) to manage versions).

## Prerequisites

- **Go 1.25+**
- **Bazel** (via bazelisk or direct install)
- **Node.js 20+** and **pnpm** (for the dashboard frontend)
- **PostgreSQL 13+** (for running the server)

## Build the server

```bash
bazel build //cmd/patchbase-server
```

The resulting binary is at `bazel-bin/cmd/patchbase-server/patchbase-server_linux_amd64` (or similar, depending on your platform).

## Build the agent

```bash
bazel build //agent/cmd/patchbase-agent
```

## Build everything

To build the server, agent, and the dashboard frontend in one go:

```bash
bazel build //...
```

## Running

You can run directly through Bazel without copying binaries around:

```bash
bazel run //cmd/patchbase-server -- serve
bazel run //cmd/patchbase-server -- migrate
```

## Development workflow

For a development setup with hot-reload and a local database, see the [development setup guide](../development/dev-setup).