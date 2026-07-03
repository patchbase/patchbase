//go:build integration_docker

// Package e2e contains docker-based end-to-end integration tests for the
// patchbase agent + server pipeline.
//
// Tests in this package are gated behind the `integration_docker` Go build
// tag and the corresponding Bazel `integration_docker` target tag. The
// default `bazel test //...` excludes them (see .bazelrc). To run them:
//
//	bazel test --test_tag_filters=integration_docker //internal/testing/e2e/...
//
// These tests require:
//   - A working Docker daemon reachable from the test process.
//   - A running test Postgres exposed on PATCHBASE_TEST_DATABASE_URL
//     (default: postgres://postgres:postgres@localhost:5433/patchbase_test?sslmode=disable).
//
// The patchbase-agent binary is provided automatically as a Bazel `data`
// dependency; set PATCHBASE_AGENT_BINARY only to override the path (e.g.
// when running via `go test` instead of `bazel test`).
package agent

import (
	"context"
	"testing"

	apitesting "go.patchbase.net/server/internal/testing"
	dockertest "go.patchbase.net/server/internal/testing/docker"
)

func TestAgentE2E_Debian(t *testing.T) {
	ctx := context.Background()
	exp := apitesting.DebianE2EExpectation()

	backend, adminToken := apitesting.NewE2EBackend(t, ctx)
	srv := apitesting.StartHTTPServer(t, backend)

	agentBin := dockertest.ResolveAgentBinaryOrSkip(t)

	// The agent refuses non-loopback HTTP URLs, so the container must run
	// with the host's network namespace. The agent then reaches the test
	// server at 127.0.0.1:<srv.URL port>.
	c := dockertest.Start(t, ctx, dockertest.StartOptions{
		Distro:       exp.Name,
		AgentBinPath: agentBin,
		HostNetwork:  true,
	})

	regToken := apitesting.CreateRegistrationToken(t, backend, adminToken, "e2e-"+exp.Name)
	dockertest.RunAgentEnroll(t, ctx, c, srv.URL, regToken)

	hostID := apitesting.ApprovePendingHost(t, backend, adminToken)
	dockertest.RunAgentSync(t, ctx, c)

	snapshotID := apitesting.LatestSnapshotID(t, backend, adminToken, hostID)

	snapshot := apitesting.FetchParsedSnapshot(t, backend, hostID)
	apitesting.AssertSnapshotMatches(t, snapshot, c.MachineID, exp)
	apitesting.AssertCurrentStatePointsAtLatestSnapshot(t, backend, hostID, snapshotID)
}
