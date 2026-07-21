// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

// Ubuntu variant of TestSSHPullE2E_Debian. See sshpull_debian_test.go for
// the rationale; this test reuses the shared e2e_helpers via
// apitesting.UbuntuE2EExpectation() so the per-distro test files stay
// thin (data + flow only).
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

func TestSSHPullE2E_Ubuntu(t *testing.T) {
	ctx := context.Background()
	exp := apitesting.UbuntuE2EExpectation()

	backend, adminToken := apitesting.NewE2EBackend(t, ctx)

	c := dockertest.Start(t, ctx, dockertest.StartOptions{Distro: exp.Name})

	hostID := dockertest.RunSSHPullForContainer(t, ctx, backend, adminToken, "e2e-sshpull-"+exp.Name, c)

	snapshot := apitesting.FetchParsedSnapshot(t, backend, hostID)
	apitesting.AssertSnapshotMatches(t, snapshot, c.MachineID, exp)
	apitesting.AssertCurrentStatePointsAtLatestSnapshot(t, backend, hostID)
}
