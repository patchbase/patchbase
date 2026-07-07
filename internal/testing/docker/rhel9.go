//go:build integration_docker

package dockertest

import (
	"context"
	"testing"
)

// StartRHEL9 brings up a Red Hat UBI 9 container with sshd running and
// returns the host connection coordinates. If agentBinPath is non-empty
// the agent binary is copied into /usr/local/bin/patchbase-agent inside
// the container so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func StartRHEL9(t *testing.T, ctx context.Context, agentBinPath string) *Container {
	return Start(t, ctx, StartOptions{Distro: "rhel9", AgentBinPath: agentBinPath})
}

// StartRHEL9WithOptions is the configurable RHEL 9 entry point used by
// StartRHEL9 and any future RHEL 9 variants.
func StartRHEL9WithOptions(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	opts.Distro = "rhel9"
	return Start(t, ctx, opts)
}

// RHEL9DockerfileDir returns the path to the directory containing the
// RHEL 9 Dockerfile. It is a convenience wrapper for DistroDockerfileDir.
func RHEL9DockerfileDir() (string, error) {
	return DistroDockerfileDir("rhel9")
}
