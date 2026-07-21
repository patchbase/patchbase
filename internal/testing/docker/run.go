// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
//go:build integration_docker

// Package dockertest provides a small testcontainers-go helper for the
// patchbase integration test suite. It builds a pinned distro image, brings
// up an ephemeral container, starts sshd inside it, and returns the
// connection coordinates the test needs.
//
// The package is distro-agnostic: callers pick a distro (e.g. "debian", "ubuntu", "rocky")
// whose Dockerfile lives under internal/testing/docker/<distro>/.
package dockertest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.patchbase.net/server/internal/services"
	apitesting "go.patchbase.net/server/internal/testing"
)

// Container describes a running test container and the connection
// information the integration tests need.
type Container struct {
	tc testcontainers.Container

	// Distro is the distro name passed to Start (e.g. "debian").
	Distro string
	// SSHAddress is host:port on the host, forwarded to the container's port 22.
	SSHAddress string
	// MachineID is the value baked into the container's /etc/machine-id.
	MachineID string
	// HostIP is the IP address the host should use to reach the container.
	HostIP string
	// HostPort is the host port forwarded to the container's port 22.
	HostPort string
	// Terminate releases the container and its image. Registered with
	// t.Cleanup automatically by Start.
	Terminate func()
}

// StartOptions customises Start.
type StartOptions struct {
	// Distro selects the Dockerfile under
	// internal/testing/docker/<distro>/Dockerfile. Required.
	Distro string
	// AgentBinPath, if non-empty, is the host path to the patchbase-agent
	// binary which is copied into the container at
	// /usr/local/bin/patchbase-agent.
	AgentBinPath string
	// HostNetwork, if true, runs the container in the host's network
	// namespace. Use this for the agent e2e test so the agent inside
	// the container can reach a host-bound httptest server at
	// 127.0.0.1. The SSH-pull test does NOT need this; the default
	// bridge networking is fine for it.
	HostNetwork bool
}

// Start brings up a container for the given distro with sshd running and
// returns the host connection coordinates. If opts.AgentBinPath is non-empty
// the agent binary is copied into /usr/local/bin/patchbase-agent inside the
// container so the test can exec it.
//
// The container is automatically terminated via t.Cleanup.
func Start(t *testing.T, ctx context.Context, opts StartOptions) *Container {
	t.Helper()
	if opts.Distro == "" {
		t.Fatalf("dockertest.Start: Distro is required")
	}

	dockerfileDir, err := DistroDockerfileDir(opts.Distro)
	if err != nil {
		t.Fatalf("resolve %s dockerfile dir: %v", opts.Distro, err)
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       dockerfileDir,
			Dockerfile:    "Dockerfile",
			KeepImage:     false,
			PrintBuildLog: false,
		},
		ExposedPorts: []string{"22/tcp"},
		Cmd:          []string{"sleep", "infinity"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.AutoRemove = false
			if opts.HostNetwork {
				hc.NetworkMode = "host"
			}
		},
	}
	// Wait for the container process to be running. We start sshd
	// ourselves via docker exec (the container's CMD is just `sleep
	// infinity`), so we must not block on the SSH port here.
	req.WaitingFor = wait.ForExec([]string{"true"}).WithStartupTimeout(2 * time.Minute)

	tc, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start %s container: %v", opts.Distro, err)
	}

	host, err := tc.Host(ctx)
	if err != nil {
		_ = tc.Terminate(context.Background())
		t.Fatalf("get container host: %v", err)
	}
	// In host network mode the container shares the host's network
	// namespace, so loopback is the host's loopback.
	if opts.HostNetwork {
		host = "127.0.0.1"
	}

	// Start sshd inside the container. When running in host network mode
	// the container shares the host's port 22 (which may or may not be
	// open), so we run sshd on a non-standard port to keep the tests
	// independent of host services. With host networking there is no
	// Docker port mapping, so we skip MappedPort entirely and use the
	// fixed sshd port directly.
	const hostNetSSHPort = "2222"
	var sshPort string
	if opts.HostNetwork {
		if _, err := execCommand(ctx, tc, "/usr/sbin/sshd", "-p", hostNetSSHPort); err != nil {
			_ = tc.Terminate(context.Background())
			t.Fatalf("start sshd on %s: %v", hostNetSSHPort, err)
		}
		sshPort = hostNetSSHPort
	} else {
		if _, err := execCommand(ctx, tc, "/usr/sbin/sshd"); err != nil {
			_ = tc.Terminate(context.Background())
			t.Fatalf("start sshd: %v", err)
		}
		mapped, err := tc.MappedPort(ctx, nat.Port("22/tcp"))
		if err != nil {
			_ = tc.Terminate(context.Background())
			t.Fatalf("get mapped ssh port: %v", err)
		}
		sshPort = mapped.Port()
	}

	// Wait for sshd to accept connections before returning. The wait
	// strategy above only confirms the container is running; sshd is
	// started via docker exec and needs a moment to bind.
	if err := waitForSSH(ctx, host, sshPort, 30*time.Second); err != nil {
		_ = tc.Terminate(context.Background())
		t.Fatalf("wait for sshd on %s:%s: %v", host, sshPort, err)
	}

	machineID, err := readMachineID(ctx, tc)
	if err != nil {
		_ = tc.Terminate(context.Background())
		t.Fatalf("read machine id: %v", err)
	}

	if opts.AgentBinPath != "" {
		if err := copyAgentBinary(ctx, tc, opts.AgentBinPath); err != nil {
			_ = tc.Terminate(context.Background())
			t.Fatalf("copy agent binary: %v", err)
		}
	}

	c := &Container{
		tc:         tc,
		Distro:     opts.Distro,
		SSHAddress: fmt.Sprintf("%s:%s", host, sshPort),
		HostIP:     host,
		HostPort:   sshPort,
		MachineID:  machineID,
		Terminate: func() {
			termCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := tc.Terminate(termCtx); err != nil {
				t.Logf("terminate %s container: %v", opts.Distro, err)
			}
		},
	}
	t.Cleanup(c.Terminate)
	return c
}

// --- Distro-specific convenience wrappers ---
//
// Per-distro wrappers live in debian.go, ubuntu.go, etc. New distros
// should follow the same pattern: add Start<Distro>(t, ctx, agentBinPath)
// and Start<Distro>WithOptions(t, ctx, opts) plus a <Distro>DockerfileDir
// helper in a sibling file in this package.

// --- Path resolution ---

// DistroDockerfileDir returns the path to the directory containing the
// Dockerfile for the given distro (e.g. "debian", "ubuntu", "rocky"). The
// Dockerfile is expected at internal/testing/docker/<distro>/Dockerfile.
//
// When running under Bazel the runfiles tree contains symlinks back to the
// real workspace sources; we resolve through them so the Docker daemon
// (which runs outside the Bazel sandbox/execroot) can access the build
// context. Under `go test` the workspace-root relative resolution is used
// as a fallback.
func DistroDockerfileDir(distro string) (string, error) {
	if dir := os.Getenv("PATCHBASE_DOCKERFILE_DIR"); dir != "" {
		return dir, nil
	}
	rel := filepath.Join("..", "..", "docker", distro)
	if info, err := os.Stat(filepath.Join(rel, "Dockerfile")); err == nil && !info.IsDir() {
		abs, err := filepath.Abs(rel)
		if err != nil {
			return "", fmt.Errorf("resolve %s dockerfile dir: %w", distro, err)
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(abs, "Dockerfile"))
		if err != nil {
			return "", fmt.Errorf("resolve %s dockerfile symlinks: %w", distro, err)
		}
		return filepath.Dir(resolved), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	candidates := []string{
		filepath.Join(cwd, "internal", "testing", "docker", distro),
	}
	dir := cwd
	for {
		candidates = append(candidates, filepath.Join(dir, "internal", "testing", "docker", distro))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "Dockerfile")); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s Dockerfile not found; set PATCHBASE_DOCKERFILE_DIR", distro)
}

// PrivateKeyDir returns the directory containing the shared test SSH
// keypair fixtures (internal/testing/docker/fixtures/). These keys are
// distro-agnostic and reused across all e2e tests. See
// fixtures/regenerate.sh to regenerate them.
func PrivateKeyDir() (string, error) {
	rel := filepath.Join("..", "..", "docker", "fixtures")
	if info, err := os.Stat(filepath.Join(rel, "id_ed25519")); err == nil && !info.IsDir() {
		abs, err := filepath.Abs(rel)
		if err != nil {
			return "", fmt.Errorf("resolve fixtures dir: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return "", fmt.Errorf("resolve fixtures dir symlinks: %w", err)
		}
		return resolved, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	candidates := []string{
		filepath.Join(cwd, "internal", "testing", "docker", "fixtures"),
	}
	dir := cwd
	for {
		candidates = append(candidates, filepath.Join(dir, "internal", "testing", "docker", "fixtures"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "id_ed25519")); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("fixtures dir not found; set PATCHBASE_DOCKERFILE_DIR")
}

// --- Agent binary resolution ---

// AgentBinaryRunfilesRelPath is the workspace-relative path to the
// patchbase-agent binary within the Bazel runfiles tree. The
// `patchbase-agent_/` segment is rules_go's conventional output
// directory for go_binary targets.
const AgentBinaryRunfilesRelPath = "agent/cmd/patchbase-agent/patchbase-agent_/patchbase-agent"

// ResolveAgentBinary locates the patchbase-agent executable. It honours
// PATCHBASE_AGENT_BINARY when set (useful for running the test outside
// Bazel, e.g. via `go test`); otherwise it resolves the binary through
// the Bazel runfiles tree (TEST_SRCDIR/TEST_WORKSPACE) where the
// `data = ["//agent/cmd/patchbase-agent:patchbase-agent"]` dependency
// stages it during `bazel test`.
func ResolveAgentBinary() (string, error) {
	if p := os.Getenv("PATCHBASE_AGENT_BINARY"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("PATCHBASE_AGENT_BINARY not found: %w", err)
		}
		return p, nil
	}
	if srcDir := os.Getenv("TEST_SRCDIR"); srcDir != "" {
		workspace := os.Getenv("TEST_WORKSPACE")
		if workspace != "" {
			p := filepath.Join(srcDir, workspace, AgentBinaryRunfilesRelPath)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("patchbase-agent not found; set PATCHBASE_AGENT_BINARY or run via `bazel test` with the agent binary in `data`")
}

// ResolveAgentBinaryOrSkip resolves the patchbase-agent binary and skips
// the test if it is not available. This is the convenience wrapper for
// agent e2e tests.
func ResolveAgentBinaryOrSkip(t *testing.T) string {
	t.Helper()
	agentBin, err := ResolveAgentBinary()
	if err != nil {
		t.Skipf("agent binary not available; skipping docker e2e test: %v", err)
	}
	t.Logf("using agent binary: %s", agentBin)
	return agentBin
}

// RunAgentEnroll runs the agent's enroll command inside the container.
// It fatals on any failure.
func RunAgentEnroll(t *testing.T, ctx context.Context, c *Container, serverURL, regToken string) {
	t.Helper()

	enrollOutput, err := c.Exec(ctx,
		"patchbase-agent", "enroll",
		"-k",
		"-c", "/etc/patchbase-agent/config.json",
		serverURL, regToken,
	)
	if err != nil {
		t.Fatalf("agent enroll: %v\noutput: %s", err, enrollOutput)
	}
	t.Logf("agent enroll output: %s", enrollOutput)
}

// RunAgentSync runs the agent's sync command inside the container.
// It fatals on any failure.
func RunAgentSync(t *testing.T, ctx context.Context, c *Container) {
	t.Helper()

	syncOutput, err := c.Exec(ctx,
		"patchbase-agent", "sync",
		"-k",
		"-c", "/etc/patchbase-agent/config.json",
	)
	if err != nil {
		t.Fatalf("agent sync: %v\noutput: %s", err, syncOutput)
	}
	t.Logf("agent sync output: %s", syncOutput)
}

// RunSSHPullForContainer creates an SSH host, injects the generated public
// key into the container, runs the SSH pull, and asserts the pull job
// succeeded. It returns the hostID. The caller passes the displayName to
// identify the test host.
func RunSSHPullForContainer(t *testing.T, ctx context.Context, backend *apitesting.Backend, adminToken, displayName string, c *Container) string {
	t.Helper()

	hostID, publicKey := apitesting.CreateUniqueSSHHost(t, backend, adminToken, displayName, c.SSHAddress)
	require.NoError(t, c.WriteAuthorizedKey(ctx, publicKey))

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	require.NoError(t, hostsService.RunSSHPull(ctx, hostID))

	jobsRec := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/pull-jobs", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, jobsRec.Code)
	var jobs []map[string]any
	require.NoError(t, json.Unmarshal(jobsRec.Body.Bytes(), &jobs))
	require.NotEmpty(t, jobs, "no ssh pull job recorded")
	assert.Equal(t, "success", jobs[0]["status"], "ssh pull job did not succeed: %v", jobs[0])

	return hostID
}

// --- Container operations ---

// Exec runs a command inside the container and returns its combined stdout
// and stderr.
func (c *Container) Exec(ctx context.Context, cmd ...string) (string, error) {
	return execCommand(ctx, c.tc, cmd...)
}

// WriteAuthorizedKey replaces /root/.ssh/authorized_keys inside the
// container with the supplied key. Used by the SSH-pull test to inject
// the per-host public key generated by the server.
func (c *Container) WriteAuthorizedKey(ctx context.Context, publicKey string) error {
	if _, err := c.Exec(ctx, "sh", "-c", fmt.Sprintf("echo %q > /root/.ssh/authorized_keys && chmod 600 /root/.ssh/authorized_keys", publicKey)); err != nil {
		return fmt.Errorf("write authorized_keys: %w", err)
	}
	return nil
}

// --- internal helpers ---

func execCommand(ctx context.Context, tc testcontainers.Container, cmd ...string) (string, error) {
	exitCode, reader, err := tc.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return "", fmt.Errorf("exec %v: %w", cmd, err)
	}
	output, _ := io.ReadAll(reader)
	if exitCode != 0 {
		return string(output), fmt.Errorf("exec %v: exit %d: %s", cmd, exitCode, string(output))
	}
	return string(output), nil
}

func readMachineID(ctx context.Context, tc testcontainers.Container) (string, error) {
	output, err := execCommand(ctx, tc, "cat", "/etc/machine-id")
	if err != nil {
		return "", fmt.Errorf("cat /etc/machine-id: %w", err)
	}
	return string(bytes.TrimSpace([]byte(output))), nil
}

func copyAgentBinary(ctx context.Context, tc testcontainers.Container, hostPath string) error {
	data, err := os.ReadFile(hostPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", hostPath, err)
	}
	if err := tc.CopyToContainer(ctx, data, "/usr/local/bin/patchbase-agent", 0o755); err != nil {
		return fmt.Errorf("copy into container: %w", err)
	}
	return nil
}

// waitForSSH polls the given host:port until a TCP connection succeeds
// or the timeout expires. This is called after starting sshd via docker
// exec to ensure it is ready before the test proceeds.
func waitForSSH(ctx context.Context, host, port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(host, port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for sshd on %s", addr)
}
