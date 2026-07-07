//go:build integration_docker

package dockertest

import (
	"context"
	"testing"
)

// StartRHEL10 brings up a Red Hat UBI 10 container with sshd running and
// returns the host connection coordinates. If agentBinPath is non-empty
// the agent binary is copied into /usr/local/bin/patchbase-agent inside
// the container so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func StartRHEL10(t *testing.T, ctx context.Context, agentBinPath string) *Container {
	return Start(t, ctx, StartOptions{Distro: "rhel10", AgentBinPath: agentBinPath})
}

// StartRHEL10WithOptions is the configurable RHEL 10 entry point used by
// StartRHEL10 and any future RHEL 10 variants.
func StartRHEL10WithOptions(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	opts.Distro = "rhel10"
	return Start(t, ctx, opts)
}

// RHEL10DockerfileDir returns the path to the directory containing the
// RHEL 10 Dockerfile. It is a convenience wrapper for DistroDockerfileDir.
func RHEL10DockerfileDir() (string, error) {
	return DistroDockerfileDir("rhel10")
}
