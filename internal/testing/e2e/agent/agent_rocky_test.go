// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

// Rocky variant of TestAgentE2E_Debian. See agent_debian_test.go for the
// rationale; this test reuses the shared e2e_helpers via
// apitesting.RockyE2EExpectation() so the per-distro test files stay
// thin (data + flow only).
//
// Run with:
//
//	bazel test --test_tag_filters=integration_docker //internal/testing/e2e/...
package agent

import (
	"context"
	"testing"

	apitesting "go.patchbase.net/server/internal/testing"
	dockertest "go.patchbase.net/server/internal/testing/docker"
)

func TestAgentE2E_Rocky(t *testing.T) {
	ctx := context.Background()
	exp := apitesting.RockyE2EExpectation()

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
