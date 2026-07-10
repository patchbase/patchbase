# Building with Bazel

PatchBase uses [Bazel](https://bazel.build) as its build system. All build, test, and run commands go through Bazel.

## Prerequisites

Install [bazelisk](https://github.com/bazelbuild/bazelisk) (recommended) or Bazel 7+ directly:

```bash
# Using Go
go install github.com/bazelbuild/bazelisk@latest

# Or download the binary
# See https://github.com/bazelbuild/bazelisk/releases
```

Bazelisk reads the Bazel version from `.bazeliskrc` and automatically downloads the right version.

## Build commands

```bash
# Build everything (server, agent, dashboard)
bazel build //...

# Build just the server
bazel build //cmd/patchbase-server

# Build just the agent
bazel build //agent/cmd/patchbase-agent

# Build the documentation site
bazel build //docs:build_docs
```

## Test commands

```bash
# Run all tests
bazel test //...

# Run tests for a specific package
bazel test //internal/services/...
bazel test //agent/internal/collector/...
```

## Run commands

```bash
# Run the server
bazel run //cmd/patchbase-server -- serve

# Run migrations
bazel run //cmd/patchbase-server -- migrate

# Run the agent
bazel run //agent/cmd/patchbase-agent -- sync
```

## Gazelle

[Gazelle](https://github.com/bazelcontrib/rules_go) generates `BUILD.bazel` files from Go source. After changing imports or adding files:

```bash
bazel run //:gazelle
```

If Gazelle adds a new `go_test` target, annotate it with `size = "small"` since Gazelle doesn't set that by default:

```python
go_test(
    name = "foo_test",
    size = "small",
    ...
)
```

## Linting

```bash
golangci-lint run
```