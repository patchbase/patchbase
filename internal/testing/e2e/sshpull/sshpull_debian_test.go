// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

// Package sshpull contains docker-based end-to-end integration tests for the
// patchbase server's SSH-pull path (defaultSSHPullRunner + the shell
// collection scripts in services/ssh_pull.go).
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
package sshpull

import (
	"context"
	"testing"

	apitesting "go.patchbase.net/server/internal/testing"
	dockertest "go.patchbase.net/server/internal/testing/docker"
)

func TestSSHPullE2E_Debian(t *testing.T) {
	ctx := context.Background()
	exp := apitesting.DebianE2EExpectation()

	backend, adminToken := apitesting.NewE2EBackend(t, ctx)

	c := dockertest.Start(t, ctx, dockertest.StartOptions{Distro: exp.Name})

	hostID := dockertest.RunSSHPullForContainer(t, ctx, backend, adminToken, "e2e-sshpull-"+exp.Name, c)

	snapshot := apitesting.FetchParsedSnapshot(t, backend, hostID)
	apitesting.AssertSnapshotMatches(t, snapshot, c.MachineID, exp)
	apitesting.AssertCurrentStatePointsAtLatestSnapshot(t, backend, hostID)
}
