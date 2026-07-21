// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

package dockertest

import (
	"context"
	"testing"
)

// StartUbuntu brings up an Ubuntu container with sshd running and returns
// the host connection coordinates. If agentBinPath is non-empty the agent
// binary is copied into /usr/local/bin/patchbase-agent inside the container
// so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func StartUbuntu(t *testing.T, ctx context.Context, agentBinPath string) *Container {
	return Start(t, ctx, StartOptions{Distro: "ubuntu", AgentBinPath: agentBinPath})
}

// StartUbuntuWithOptions is the configurable Ubuntu entry point used by
// StartUbuntu and any future Ubuntu variants.
func StartUbuntuWithOptions(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	opts.Distro = "ubuntu"
	return Start(t, ctx, opts)
}

// UbuntuDockerfileDir returns the path to the directory containing the
// Ubuntu Dockerfile. It is a convenience wrapper for DistroDockerfileDir.
func UbuntuDockerfileDir() (string, error) {
	return DistroDockerfileDir("ubuntu")
}
