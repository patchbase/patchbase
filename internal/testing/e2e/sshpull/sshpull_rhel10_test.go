//go:build integration_docker

// Red Hat UBI 10 variant of TestSSHPullE2E_Debian. See
// sshpull_debian_test.go for the rationale; this test reuses the shared
// e2e_helpers via apitesting.RHEL10E2EExpectation() so the per-distro test
// files stay thin (data + flow only).
//
// Run with:
//
//	bazel test --test_tag_filters=integration_docker //internal/testing/e2e/...
package sshpull

import (
	"context"
	"testing"

	apitesting "go.patchbase.net/server/internal/testing"
	dockertest "go.patchbase.net/server/internal/testing/docker"
)

func TestSSHPullE2E_RHEL10(t *testing.T) {
	ctx := context.Background()
	exp := apitesting.RHEL10E2EExpectation()

	backend, adminToken := apitesting.NewE2EBackend(t, ctx)

	c := dockertest.Start(t, ctx, dockertest.StartOptions{Distro: exp.Name})

	hostID := dockertest.RunSSHPullForContainer(t, ctx, backend, adminToken, "e2e-sshpull-"+exp.Name, c)

	snapshot := apitesting.FetchParsedSnapshot(t, backend, hostID)
	apitesting.AssertSnapshotMatches(t, snapshot, c.MachineID, exp)
	apitesting.AssertCurrentStatePointsAtLatestSnapshot(t, backend, hostID)
}
