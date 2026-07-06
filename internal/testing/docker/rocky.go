//go:build integration_docker

package dockertest

import (
	"context"
	"testing"
)

// StartRocky brings up a Rocky Linux container with sshd running and returns
// the host connection coordinates. If agentBinPath is non-empty the agent
// binary is copied into /usr/local/bin/patchbase-agent inside the container
// so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func StartRocky(t *testing.T, ctx context.Context, agentBinPath string) *Container {
	return Start(t, ctx, StartOptions{Distro: "rocky", AgentBinPath: agentBinPath})
}

// StartRockyWithOptions is the configurable Rocky entry point used by
// StartRocky and any future Rocky variants.
func StartRockyWithOptions(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	opts.Distro = "rocky"
	return Start(t, ctx, opts)
}

// RockyDockerfileDir returns the path to the directory containing the
// Rocky Linux Dockerfile. It is a convenience wrapper for DistroDockerfileDir.
func RockyDockerfileDir() (string, error) {
	return DistroDockerfileDir("rocky")
}
