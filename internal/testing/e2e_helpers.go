package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	gotesting "testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"google.golang.org/protobuf/proto"
)

// --- Distro expectations ---

// DistroExpectation describes the expected snapshot properties for a
// distro. Each distro test declares one of these and passes it to the
// assertion helpers, keeping per-distro test files to just data + flow.
type DistroExpectation struct {
	// Name is the distro identifier used to select the Dockerfile
	// (e.g. "debian", "ubuntu", "rocky").
	Name string
	// OSFamily expected in the snapshot.
	OSFamily agentpb.OsFamily
	// OSName expected in the snapshot.
	OSName string
	// OSVersion expected in the snapshot.
	OSVersion string
	// Architecture expected in the snapshot.
	Architecture agentpb.Architecture
	// Package that must be present in the snapshot.
	Package PackageExpectation
	// Repo that must be present in the snapshot (optional).
	Repo RepoExpectation
}

// PackageExpectation describes a single package that must appear in the
// snapshot with the given name and (if non-empty) version + release.
type PackageExpectation struct {
	Name    string
	Version string
	Release string
}

// RepoExpectation describes a repository that must appear in the snapshot.
// If BaseURL is non-empty it is matched against repo.baseurl. Labels is
// matched against repo.repo_label (any label matching is sufficient).
type RepoExpectation struct {
	BaseURL string
	Labels  []string
}

// DebianE2EExpectation returns the expectation for the pinned Debian 12
// test image (see internal/testing/docker/debian/Dockerfile).
func DebianE2EExpectation() DistroExpectation {
	return DistroExpectation{
		Name:         "debian",
		OSFamily:     agentpb.OsFamily_OS_FAMILY_APT,
		OSName:       "Debian GNU/Linux",
		OSVersion:    "12",
		Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		Package: PackageExpectation{
			Name:    "nginx",
			Version: "1.22.1",
			Release: "9+deb12u6",
		},
		Repo: RepoExpectation{
			BaseURL: "http://snapshot.debian.org/archive/debian/20260701T000000Z",
			Labels:  []string{"bookworm main", "bookworm"},
		},
	}
}

// --- Backend / admin setup ---

// NewE2EBackend creates a test backend with the standard user fixture
// loaded and returns the backend plus an admin access token. This is the
// common setup shared by all e2e tests.
func NewE2EBackend(t *gotesting.T, ctx context.Context) (*Backend, string) {
	t.Helper()
	backend := NewBackend(
		t,
		WithFixture(LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(ctx, "u_admin")
	require.NoError(t, err)
	return backend, adminToken
}

// StartHTTPServer wraps the backend's ServeMux in an httptest.Server.
// Only the agent e2e path needs this — the SSH-pull path drives the
// server directly. The server is registered for cleanup via t.Cleanup.
func StartHTTPServer(t *gotesting.T, backend *Backend) *httptest.Server {
	t.Helper()
	mux := do.MustInvoke[*http.ServeMux](backend.Injector())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- API helpers ---

// CreateRegistrationToken creates a host registration token via the HTTP
// API and returns the token string.
func CreateRegistrationToken(t gotesting.TB, backend *Backend, adminToken, name string) string {
	t.Helper()
	rec := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		fmt.Sprintf(`{"name":%q}`, name),
		WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec.Code, "create registration token: %s", rec.Body.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	token, _ := payload["token"].(string)
	require.NotEmpty(t, token)
	return token
}

// ApprovePendingHost approves the first pending host via the HTTP API and
// returns its host ID.
func ApprovePendingHost(t gotesting.TB, backend *Backend, adminToken string) string {
	t.Helper()
	listRec := backend.HTTPGet("/api/v1/hosts/pending", WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, listRec.Code)
	var pending []map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &pending))
	require.NotEmpty(t, pending, "no pending host found")
	hostID, _ := pending[0]["id"].(string)
	require.NotEmpty(t, hostID)

	approveRec := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
		"{}",
		WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRec.Code)
	return hostID
}

// LatestSnapshotID fetches the latest snapshot metadata via the HTTP API
// and returns the snapshot ID.
func LatestSnapshotID(t gotesting.TB, backend *Backend, adminToken, hostID string) string {
	t.Helper()
	rec := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/snapshot", hostID),
		WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snapshot))
	snapshotID, _ := snapshot["id"].(string)
	require.NotEmpty(t, snapshotID, "snapshot response missing id: %#v", snapshot)
	return snapshotID
}

// CreateUniqueSSHHost creates an SSH host with a unique key pair via the
// HTTP API and returns the host ID and the generated public key.
func CreateUniqueSSHHost(t gotesting.TB, backend *Backend, adminToken, displayName, sshAddress string) (hostID, publicKey string) {
	t.Helper()
	rec := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		fmt.Sprintf(
			`{"display_name":%q,"hostname":%q,"ssh_user":"root","frequency_minutes":60,"unique_key_pair":true}`,
			displayName, sshAddress,
		),
		WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec.Code, "create ssh host: %s", rec.Body.String())
	var payload struct {
		HostID    string `json:"host_id"`
		PublicKey string `json:"public_key"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.NotEmpty(t, payload.HostID, "create response missing host_id: %s", rec.Body.String())
	require.NotEmpty(t, payload.PublicKey, "create response missing public_key")
	return payload.HostID, payload.PublicKey
}

// FetchSnapshotPayload returns the raw payload bytes stored for the host's
// latest snapshot, queried directly from the test database. The HTTP
// `GetLatestSnapshot` endpoint does not expose the payload field.
//
// This is shared by all e2e integration tests (agent, ssh-pull, etc.)
// regardless of distro.
func FetchSnapshotPayload(t gotesting.TB, backend *Backend, hostID string) []byte {
	t.Helper()
	var payload []byte
	row := backend.DB().QueryRow(
		context.Background(),
		`SELECT payload FROM host_snapshots WHERE host_id = $1 ORDER BY collected_at DESC LIMIT 1`,
		hostID,
	)
	require.NoError(t, row.Scan(&payload), "scan host_snapshots.payload for %s", hostID)
	return payload
}

// FetchParsedSnapshot retrieves the raw snapshot payload from the database
// and unmarshals it into an AgentSnapshot proto.
func FetchParsedSnapshot(t gotesting.TB, backend *Backend, hostID string) *agentpb.AgentSnapshot {
	t.Helper()
	payload := FetchSnapshotPayload(t, backend, hostID)
	parsed := &agentpb.AgentSnapshot{}
	require.NoError(t, proto.Unmarshal(payload, parsed))
	return parsed
}

// --- Snapshot assertions ---

// AssertSnapshotHost asserts that the snapshot's host metadata matches the
// distro expectation and the container's machine ID.
func AssertSnapshotHost(t gotesting.TB, snapshot *agentpb.AgentSnapshot, containerMachineID string, exp DistroExpectation) {
	t.Helper()
	require.NotNil(t, snapshot.GetHost())
	assert.Equal(t, exp.OSFamily, snapshot.GetHost().GetOsFamily())
	assert.Equal(t, exp.OSName, snapshot.GetHost().GetOsName())
	assert.Equal(t, exp.OSVersion, snapshot.GetHost().GetOsVersion())
	assert.Equal(t, containerMachineID, snapshot.GetHost().GetMachineId())
	assert.Equal(t, exp.Architecture, snapshot.GetHost().GetArchitecture())
	assert.NotEmpty(t, snapshot.GetRuntime().GetKernelRunning(), "kernel running is empty")
}

// AssertSnapshotHasPackage asserts that the snapshot contains the expected
// package with matching version and release (if specified).
func AssertSnapshotHasPackage(t gotesting.TB, snapshot *agentpb.AgentSnapshot, exp PackageExpectation) {
	t.Helper()
	for _, pkg := range snapshot.GetPackages() {
		if pkg.GetName() == exp.Name {
			if exp.Version != "" {
				assert.Equal(t, exp.Version, pkg.GetVersion(),
					"%s version mismatch; was the Dockerfile snapshot date bumped without updating assertions?",
					exp.Name)
			}
			if exp.Release != "" {
				assert.Equal(t, exp.Release, pkg.GetRelease(),
					"%s release mismatch; was the Dockerfile snapshot date bumped without updating assertions?",
					exp.Name)
			}
			return
		}
	}
	assert.Fail(t, fmt.Sprintf("%s package missing from snapshot", exp.Name))
}

// AssertSnapshotHasRepo asserts that the snapshot contains a repository
// matching the expectation (by base URL or any of the labels).
func AssertSnapshotHasRepo(t gotesting.TB, snapshot *agentpb.AgentSnapshot, exp RepoExpectation) {
	t.Helper()
	if exp.BaseURL == "" && len(exp.Labels) == 0 {
		assert.NotEmpty(t, snapshot.GetRepos(), "repos is empty")
		return
	}
	for _, repo := range snapshot.GetRepos() {
		if exp.BaseURL != "" && repo.GetBaseurl() == exp.BaseURL {
			return
		}
		for _, label := range exp.Labels {
			if repo.GetRepoLabel() == label {
				return
			}
		}
	}
	assert.Fail(t, fmt.Sprintf("repo matching expectation not found (base_url=%s, labels=%v)", exp.BaseURL, exp.Labels))
}

// AssertSnapshotMatches is a convenience wrapper that runs all snapshot
// assertions against the distro expectation.
func AssertSnapshotMatches(t gotesting.TB, snapshot *agentpb.AgentSnapshot, containerMachineID string, exp DistroExpectation) {
	t.Helper()
	AssertSnapshotHost(t, snapshot, containerMachineID, exp)
	AssertSnapshotHasPackage(t, snapshot, exp.Package)
	AssertSnapshotHasRepo(t, snapshot, exp.Repo)
}

// --- Database assertions ---

// AssertCurrentStatePointsAtLatestSnapshot asserts that
// host_current_state.snapshot_id matches the latest host_snapshots.id for
// the given host. The snapshotID parameter (from the HTTP API) is also
// checked against the current-state row when non-empty.
func AssertCurrentStatePointsAtLatestSnapshot(t gotesting.TB, backend *Backend, hostID string, snapshotID ...string) {
	t.Helper()
	ctx := context.Background()

	stateRow := backend.DB().QueryRow(ctx,
		`SELECT snapshot_id FROM host_current_state WHERE host_id = $1`, hostID)
	var stateSnapshotID string
	require.NoError(t, stateRow.Scan(&stateSnapshotID), "scan host_current_state for %s", hostID)
	assert.NotEmpty(t, stateSnapshotID, "host_current_state.snapshot_id is empty")

	latestRow := backend.DB().QueryRow(ctx,
		`SELECT id FROM host_snapshots WHERE host_id = $1 ORDER BY collected_at DESC LIMIT 1`, hostID)
	var latestSnapshotID string
	require.NoError(t, latestRow.Scan(&latestSnapshotID), "scan latest host_snapshots.id for %s", hostID)
	assert.Equal(t, latestSnapshotID, stateSnapshotID,
		"host_current_state.snapshot_id does not match the latest snapshot")

	if len(snapshotID) > 0 && snapshotID[0] != "" {
		assert.Equal(t, snapshotID[0], stateSnapshotID,
			"host_current_state.snapshot_id does not match the HTTP snapshot id")
	}
}
