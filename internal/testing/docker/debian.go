// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

package dockertest

import (
	"context"
	"testing"
)

// StartDebian brings up a Debian container with sshd running and returns
// the host connection coordinates. If agentBinPath is non-empty the agent
// binary is copied into /usr/local/bin/patchbase-agent inside the container
// so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func StartDebian(t *testing.T, ctx context.Context, agentBinPath string) *Container {
	return Start(t, ctx, StartOptions{Distro: "debian", AgentBinPath: agentBinPath})
}

// StartDebianWithOptions is the configurable Debian entry point used by
// StartDebian and any future distro variants.
func StartDebianWithOptions(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	opts.Distro = "debian"
	return Start(t, ctx, opts)
}

// DebianDockerfileDir returns the path to the directory containing the
// Debian Dockerfile. It is a convenience wrapper for DistroDockerfileDir.
func DebianDockerfileDir() (string, error) {
	return DistroDockerfileDir("debian")
}
