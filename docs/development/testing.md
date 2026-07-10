# Testing

PatchBase uses standard Go testing with `testify` for assertions.

## Running tests

```bash
# All tests
bazel test //...

# Specific package
bazel test //internal/services/...
bazel test //agent/internal/collector/...

# With verbose output
bazel test //internal/services/... --test_arg=-v
```

## Test conventions

- Use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` instead of `t.Fatalf`-style assertions.
- Integration tests that need PostgreSQL use the test database on port 5433 (see `compose.test.yaml`).
- Unit tests should be annotated with `size = "small"` in `BUILD.bazel`.
- Name unused parameters `_` in the function signature, not in the body.

## Test database

Integration tests expect PostgreSQL on port 5433:

```bash
docker compose -f compose.test.yaml up -d
```

If you change the schema:

```bash
docker compose -f compose.test.yaml down -v
docker compose -f compose.test.yaml up -d
bazel run //cmd/patchbase-server -- migrate \
  --database-url "postgres://postgres:postgres@localhost:5433/patchbase_test?sslmode=disable"
```

## Test fixtures

Database fixtures can be loaded using `github.com/go-testfixtures/testfixtures/v3`. See existing integration tests for patterns.

## Mocks

Mocks are generated under `internal/mock/`. The test infrastructure uses `samber/do` for dependency injection, which makes it straightforward to swap implementations in tests.