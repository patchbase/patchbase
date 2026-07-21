// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

package dockertest

import (
	"context"
	"testing"
)

// StartAlma brings up an AlmaLinux container with sshd running and returns
// the host connection coordinates. If agentBinPath is non-empty the agent
// binary is copied into /usr/local/bin/patchbase-agent inside the container
// so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func StartAlma(t *testing.T, ctx context.Context, agentBinPath string) *Container {
	return Start(t, ctx, StartOptions{Distro: "alma", AgentBinPath: agentBinPath})
}

// StartAlmaWithOptions is the configurable Alma entry point used by
// StartAlma and any future Alma variants.
func StartAlmaWithOptions(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	opts.Distro = "alma"
	return Start(t, ctx, opts)
}

// AlmaDockerfileDir returns the path to the directory containing the
// AlmaLinux Dockerfile. It is a convenience wrapper for DistroDockerfileDir.
func AlmaDockerfileDir() (string, error) {
	return DistroDockerfileDir("alma")
}
