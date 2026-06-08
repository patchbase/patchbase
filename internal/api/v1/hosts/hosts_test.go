package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/services"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockAPISSHPullRunner struct {
	services.SSHPullRunner
	result     services.SSHPullResult
	err        error
	calledHost string
}

func (m *mockAPISSHPullRunner) Collect(_ context.Context, _ string, _ string, host string) (services.SSHPullResult, error) {
	m.calledHost = host
	return m.result, m.err
}

func mockAPISnapshotPayload(t *testing.T) []byte {
	t.Helper()

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			Hostname:                    "ssh-api-test-host",
			MachineId:                   "ssh-api-machine-id",
			IpAddresses:                 []string{"10.0.0.55"},
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:                      "Rocky Linux",
			OsMajor:                     9,
			OsVersion:                   "9.5",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 1,
		},
		Runtime: &agentpb.Runtime{
			KernelRunning: "5.14.0",
		},
	}

	payload, err := proto.Marshal(snapshot)
	require.NoError(t, err)
	return payload
}

func TestRegistrationTokenLifecycle(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"reg-token-1"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &created))
	tokenID := created["id"].(string)
	require.NotEmpty(t, created["token"])

	listRecorder := backend.HTTPGet("/api/v1/hosts/tokens", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, listRecorder.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.NotEmpty(t, listed)
	assert.Equal(t, tokenID, listed[0]["id"])

	revokeRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/tokens/%s/revoke", tokenID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	assert.Equal(t, http.StatusOK, revokeRecorder.Code)
}

func TestHostFleetEndpoints(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"fleet-token"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &created))
	registrationToken := created["token"].(string)

	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: registrationToken,
		Hostname:          "fleet-host",
		MachineId:         "fleet-machine",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.44",
			OsName:       "linux",
			OsVersion:    "9.5",
			Architecture: "x86_64",
		},
	}
	registerReqBytes, err := proto.Marshal(registerReq)
	require.NoError(t, err)

	registerRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/register",
		registerReqBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
	)
	require.Equal(t, http.StatusCreated, registerRecorder.Code)

	var registered agentpb.RegisterHostResponse
	require.NoError(t, proto.Unmarshal(registerRecorder.Body.Bytes(), &registered))
	hostID := registered.HostId
	hostAccessToken := registered.HostAccessToken

	pendingRecorder := backend.HTTPGet("/api/v1/hosts/pending", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, pendingRecorder.Code)
	var pendingPayload []map[string]any
	require.NoError(t, json.Unmarshal(pendingRecorder.Body.Bytes(), &pendingPayload))
	require.NotEmpty(t, pendingPayload)
	assert.Equal(t, "linux", pendingPayload[0]["os_name"])
	assert.Equal(t, "9.5", pendingPayload[0]["os_version"])
	assert.Equal(t, "x86_64", pendingPayload[0]["architecture"])
	assert.Equal(t, "10.0.0.44", pendingPayload[0]["ip_address"])

	approveRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder.Code)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "fleet-machine",
			Hostname:                    "fleet-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:                      "Rocky Linux",
			OsMajor:                     9,
			OsVersion:                   "9.5",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 5,
			IpAddresses:                 []string{"10.0.0.44"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-5.14.0"},
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	ingestRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostAccessToken),
	)
	require.Equal(t, http.StatusAccepted, ingestRecorder.Code)

	var acceptedPayload agentpb.SyncResponse
	require.NoError(t, proto.Unmarshal(ingestRecorder.Body.Bytes(), &acceptedPayload))
	assert.Equal(t, true, acceptedPayload.Accepted)
	assert.NotEmpty(t, acceptedPayload.SnapshotId)

	listHostsRecorder := backend.HTTPGet("/api/v1/hosts", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, listHostsRecorder.Code)
	var hostsPayload []map[string]any
	require.NoError(t, json.Unmarshal(listHostsRecorder.Body.Bytes(), &hostsPayload))
	require.NotEmpty(t, hostsPayload)

	hostRecorder := backend.HTTPGet(fmt.Sprintf("/api/v1/hosts/%s", hostID), apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, hostRecorder.Code)
	var hostPayload map[string]any
	require.NoError(t, json.Unmarshal(hostRecorder.Body.Bytes(), &hostPayload))
	assert.Equal(t, hostID, hostPayload["id"])
	assert.Equal(t, "approved", hostPayload["approval_status"])

	snapshotRecorder := backend.HTTPGet(fmt.Sprintf("/api/v1/hosts/%s/snapshot", hostID), apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, snapshotRecorder.Code)
	var snapshotPayload map[string]any
	require.NoError(t, json.Unmarshal(snapshotRecorder.Body.Bytes(), &snapshotPayload))
	assert.Equal(t, hostID, snapshotPayload["host_id"])

	deleteRecorder := backend.HTTPDelete(fmt.Sprintf("/api/v1/hosts/%s", hostID), apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, deleteRecorder.Code)

	hostAfterDeleteRecorder := backend.HTTPGet(fmt.Sprintf("/api/v1/hosts/%s", hostID), apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusNotFound, hostAfterDeleteRecorder.Code)

	listHostsAfterDeleteRecorder := backend.HTTPGet("/api/v1/hosts", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, listHostsAfterDeleteRecorder.Code)
	var hostsAfterDeletePayload []map[string]any
	require.NoError(t, json.Unmarshal(listHostsAfterDeleteRecorder.Body.Bytes(), &hostsAfterDeletePayload))
	for _, hostItem := range hostsAfterDeletePayload {
		assert.NotEqual(t, hostID, hostItem["id"])
	}
}

func TestCreateSSHHost(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"ssh-host","hostname":"203.0.113.10","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	hostID := payload["host_id"].(string)
	assert.NotEmpty(t, hostID)
	assert.NotEmpty(t, payload["public_key"])
	assert.Equal(t, "approved", payload["approval_status"])
	assert.Empty(t, payload["last_run_status"])

	// Test unique_key_pair = true
	recorderUnique := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"ssh-host-unique","hostname":"203.0.113.11","ssh_user":"root","frequency_minutes":60,"unique_key_pair":true}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, recorderUnique.Code)

	var payloadUnique map[string]any
	require.NoError(t, json.Unmarshal(recorderUnique.Body.Bytes(), &payloadUnique))
	assert.NotEmpty(t, payloadUnique["host_id"])
	assert.NotEmpty(t, payloadUnique["public_key"])
	assert.NotEqual(t, payload["public_key"], payloadUnique["public_key"])

	// Test onboarding endpoint
	onboardRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/onboard-ssh", hostID),
		`{}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusNoContent, onboardRecorder.Code)
}

func TestRunPullNow(t *testing.T) {
	mockRunner := &mockAPISSHPullRunner{
		result: services.SSHPullResult{
			MachineID:        "ssh-api-machine-id",
			Hostname:         "ssh-api-test-host",
			IPAddress:        "10.0.0.55",
			OSFamily:         "rpm",
			OSName:           "Rocky Linux",
			OSVersion:        "9.5",
			OSMajor:          9,
			Architecture:     "x86_64",
			RunningKernel:    "5.14.0",
			CollectedAt:      time.Now().UTC(),
			AvailableUpdates: 1,
			Payload:          mockAPISnapshotPayload(t),
			OverallAction:    "update_package",
		},
	}

	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.SSHPullRunner](i, func(_ do.Injector) (services.SSHPullRunner, error) {
				return mockRunner, nil
			})
		}),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createRecorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"ssh-pull-now","hostname":"203.0.113.10","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &payload))
	hostID := payload["host_id"].(string)

	runRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/pull-now", hostID),
		`{}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusNoContent, runRecorder.Code)
	assert.Equal(t, "203.0.113.10:22", mockRunner.calledHost)

	jobsRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/pull-jobs", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, jobsRecorder.Code)
	var jobs []map[string]any
	require.NoError(t, json.Unmarshal(jobsRecorder.Body.Bytes(), &jobs))
	require.NotEmpty(t, jobs)
	assert.Equal(t, "success", jobs[0]["status"])
}

func TestListPullJobsReturnsLastTenEntries(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createRecorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"ssh-pull-history","hostname":"203.0.113.11","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &payload))
	hostID := payload["host_id"].(string)

	baseStartedAt := time.Now().UTC().Add(-12 * time.Minute)
	for i := 0; i < 12; i++ {
		_, err := backend.DB().Exec(context.Background(), `
			INSERT INTO host_ssh_pull_jobs (id, host_id, status, started_at)
			VALUES ($1, $2, 'success', $3)
		`, fmt.Sprintf("j_%02d", i), hostID, baseStartedAt.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	jobsRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/pull-jobs", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, jobsRecorder.Code)
	var jobs []map[string]any
	require.NoError(t, json.Unmarshal(jobsRecorder.Body.Bytes(), &jobs))
	require.Len(t, jobs, 10)
	assert.Equal(t, "j_11", jobs[0]["id"])
	assert.Equal(t, "j_02", jobs[9]["id"])
	for _, job := range jobs {
		assert.NotEqual(t, "j_00", job["id"])
		assert.NotEqual(t, "j_01", job["id"])
	}
}

func TestRunPullNowHostNotFound(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	runRecorder := backend.HTTPPost(
		"/api/v1/hosts/h_missing/pull-now",
		`{}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusNotFound, runRecorder.Code)
}

func TestGetCollectorScriptIncludesContentLength(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		"/api/v1/hosts/manual/script?os_family=apt",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	assert.Equal(t, `attachment; filename="patchbase-collector.sh"`, recorder.Header().Get("Content-Disposition"))
	assert.Equal(t, "text/x-shellscript", recorder.Header().Get("Content-Type"))
	assert.Equal(t, strconv.Itoa(recorder.Body.Len()), recorder.Header().Get("Content-Length"))
	assert.NotEmpty(t, recorder.Body.String())
}

func TestGetCollectorScriptRequiresOSFamily(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		"/api/v1/hosts/manual/script",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "missing required query parameter: os_family")
}

func TestGetCollectorScriptRejectsUnsupportedOSFamily(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		"/api/v1/hosts/manual/script?os_family=solaris",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unsupported os family")
}

func TestHostVulnerableAndUpgradablePackages(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Create registration token
	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"fleet-token"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &created))
	registrationToken := created["token"].(string)

	// Register host
	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: registrationToken,
		Hostname:          "matching-host",
		MachineId:         "matching-machine",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.45",
			OsName:       "Rocky Linux",
			OsVersion:    "9.5",
			Architecture: "x86_64",
		},
	}
	registerReqBytes, err := proto.Marshal(registerReq)
	require.NoError(t, err)

	registerRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/register",
		registerReqBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
	)
	require.Equal(t, http.StatusCreated, registerRecorder.Code)

	var registered agentpb.RegisterHostResponse
	require.NoError(t, proto.Unmarshal(registerRecorder.Body.Bytes(), &registered))
	hostID := registered.HostId
	hostAccessToken := registered.HostAccessToken

	// Approve host
	approveRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder.Code)

	// Query vulnerable/upgradable packages when there's no snapshot
	vulnEmptyRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, vulnEmptyRecorder.Code)
	var vulnEmpty []any
	require.NoError(t, json.Unmarshal(vulnEmptyRecorder.Body.Bytes(), &vulnEmpty))
	assert.Empty(t, vulnEmpty)

	// Ingest a snapshot
	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "matching-machine",
			Hostname:                    "matching-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:                      "Rocky Linux",
			OsMajor:                     9,
			OsVersion:                   "9.5",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 2,
			IpAddresses:                 []string{"10.0.0.45"},
		},
		Packages: []*agentpb.Package{
			{
				Name:      "curl",
				Epoch:     0,
				Version:   "7.76.1",
				Release:   "14.el9",
				Arch:      "x86_64",
				SourceRpm: "curl-7.76.1-14.el9.src.rpm",
				Nevra:     "curl-0:7.76.1-14.el9.x86_64",
			},
		},
		UpgradablePackages: []*agentpb.Package{
			{
				Name:       "curl",
				Epoch:      0,
				Version:    "7.76.1",
				Release:    "15.el9",
				Arch:       "x86_64",
				RepoOrigin: "baseos",
				Nevra:      "curl-0:7.76.1-15.el9.x86_64",
			},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-5.14.0"},
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	ingestRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostAccessToken),
	)
	require.Equal(t, http.StatusAccepted, ingestRecorder.Code)

	// Query vulnerable and upgradable endpoints.
	// Vulnerable stays empty (no advisories), while upgradable should return host-observed updates.
	vulnRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, vulnRecorder.Code)
	var vulnGroups []map[string]any
	require.NoError(t, json.Unmarshal(vulnRecorder.Body.Bytes(), &vulnGroups))
	assert.Empty(t, vulnGroups)

	upgRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/upgradable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, upgRecorder.Code)
	var upgGroups []map[string]any
	require.NoError(t, json.Unmarshal(upgRecorder.Body.Bytes(), &upgGroups))
	assert.NotEmpty(t, upgGroups)
}

func TestHostVulnerableAndUpgradablePackages_NonEmptyAndValidation(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Seed product stream, advisory, and rule in Postgres
	ctx := context.Background()
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active')
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ('RLSA-2023:9999', 'rocky_errata_api', '9999', 'rocky', 'security', 'critical', 'Vulnerability', 'vendor_db', true)
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ('RLSA-2023:9999', 'rocky:9-baseos')
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, evidence_tier)
		VALUES ('rule_9999', 'RLSA-2023:9999', 'rocky:9-baseos', 'openssl', '< 0:3.0.7-2.el9', 'vendor_db')
	`)
	require.NoError(t, err)

	// Create and approve a Rocky 9 host
	queries := db.New(backend.DB())
	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("Rocky9-Host"),
		MachineID:    utils.Some("mach-rocky-9"),
		Hostname:     utils.Some("rocky9-test"),
		IpAddress:    utils.Some("10.0.0.98"),
		OsName:       "Rocky Linux",
		OsVersion:    "9.3",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	// Ingest snapshot for Rocky 9 host with vulnerable openssl
	hostToken := "pb_host_token987654321012"
	_, err = queries.InsertHostAccessToken(ctx, db.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    hostID,
		TokenHash: utils.SHA256(hostToken),
	})
	require.NoError(t, err)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			MachineId:    "mach-rocky-9",
			Hostname:     "rocky9-test",
			OsFamily:     agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:       "Rocky Linux",
			OsMajor:      9,
			OsVersion:    "9.3",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "openssl",
				Epoch:   0,
				Version: "3.0.7",
				Release: "1.el9",
				Arch:    "x86_64",
				Nevra:   "openssl-0:3.0.7-1.el9.x86_64",
			},
		},
		Repos: []*agentpb.Repo{
			{
				RepoId:  "baseos",
				Enabled: true,
			},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "5.14.0"},
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	ingestRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostToken),
	)
	require.Equal(t, http.StatusAccepted, ingestRecorder.Code)

	// Verify non-empty response for vulnerable packages
	vulnRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, vulnRecorder.Code)

	var vulnGroups []map[string]any
	require.NoError(t, json.Unmarshal(vulnRecorder.Body.Bytes(), &vulnGroups))
	assert.NotEmpty(t, vulnGroups)
	assert.Equal(t, "openssl", vulnGroups[0]["family_label"])

	// Verify empty response for upgradable packages (security advisories stay under vulnerable tab).
	upgRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/upgradable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, upgRecorder.Code)

	var upgGroups []map[string]any
	require.NoError(t, json.Unmarshal(upgRecorder.Body.Bytes(), &upgGroups))
	assert.Empty(t, upgGroups)

	// Verify error-path (non-existent host ID returns 404)
	errRecorder := backend.HTTPGet(
		"/api/v1/hosts/non-existent-host-id/packages/vulnerable",
		apitesting.WithBearerToken(adminToken),
	)
	assert.Equal(t, http.StatusNotFound, errRecorder.Code)
}

func TestHostKernelPosture(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	ctx := context.Background()
	queries := db.New(backend.DB())

	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("kernel-host"),
		MachineID:    utils.Some("kernel-machine"),
		Hostname:     utils.Some("kernel-host"),
		IpAddress:    utils.Some("10.0.0.47"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:    "kernel-machine",
			Hostname:     "kernel-host",
			OsFamily:     agentpb.OsFamily_OS_FAMILY_APT,
			OsName:       "Ubuntu",
			OsMajor:      22,
			OsVersion:    "22.04",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "linux-image-5.15.0-156-generic",
				Epoch:   0,
				Version: "5.15.0",
				Release: "156.166",
				Arch:    "amd64",
				Nevra:   "linux-image-5.15.0-156-generic-0:5.15.0-156.166.amd64",
			},
			{
				Name:    "linux-image-5.15.0-176-generic",
				Epoch:   0,
				Version: "5.15.0",
				Release: "176.186",
				Arch:    "amd64",
				Nevra:   "linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64",
			},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "5.15.0-156-generic"},
	}

	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	snapshotID := id.New("snap")
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, now(), $3, '5.15.0-156-generic', NULL, false)
	`, snapshotID, hostID, payloadBytes)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES
			('USN-7000-1', 'ubuntu_usn_api', '7000-1', 'ubuntu', 'security', 'critical', 'Kernel issue active only', 'vendor_db', true, now()),
			('USN-7000-2', 'ubuntu_usn_api', '7000-2', 'ubuntu', 'security', 'important', 'Kernel issue still in latest', 'vendor_db', true, now())
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisory_references (id, advisory_id, ref_type, ref_value, url)
		VALUES
			($1, 'USN-7000-1', 'cve', 'CVE-2026-1111', 'https://example.com/CVE-2026-1111'),
			($2, 'USN-7000-2', 'cve', 'CVE-2026-2222', 'https://example.com/CVE-2026-2222')
	`, id.New("aref"), id.New("aref"))
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-7000-1",
		PackageName:    "linux-image-5.15.0-156-generic",
		InstalledNevra: utils.Some("linux-image-5.15.0-156-generic-0:5.15.0-156.166.amd64"),
		FixedNevra:     utils.Some("linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64"),
		Status:         "fixed_package_installed_pending_activation",
		Action:         "reboot_host",
		Severity:       utils.Some("critical"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "fixed_package_installed_kernel_not_running",
		ReasonText:     utils.Some("fixed kernel package is installed but the running kernel is older"),
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-7000-2",
		PackageName:    "linux-image-5.15.0-176-generic",
		InstalledNevra: utils.Some("linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64"),
		FixedNevra:     utils.Some("linux-image-5.15.0-178-generic-0:5.15.0-178.188.amd64"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.Some("important"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ReasonText:     utils.Some("a vendor fixed package is available but not installed"),
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/kernel-posture", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	assert.Equal(t, "5.15.0-156-generic", payload["running_kernel"])
	assert.Equal(t, "linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64", payload["latest_installed_kernel"])
	assert.Equal(t, true, payload["reboot_would_reduce_cve_count"])

	activeKernel := payload["active_kernel"].(map[string]any)
	assert.Equal(t, float64(2), activeKernel["advisory_count"])
	assert.Equal(t, float64(2), activeKernel["cve_count"])

	latestInstalled := payload["latest_installed"].(map[string]any)
	assert.Equal(t, float64(1), latestInstalled["advisory_count"])
	assert.Equal(t, float64(1), latestInstalled["cve_count"])
}

func TestHostVulnerablePackages_SeverityFallbackFromAdvisory(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	ctx := context.Background()
	queries := db.New(backend.DB())

	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("severity-fallback-host"),
		MachineID:    utils.Some("severity-fallback-machine"),
		Hostname:     utils.Some("severity-fallback-host"),
		IpAddress:    utils.Some("10.0.0.58"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)
	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	snapshotID := id.New("snap")
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, now(), $3, '5.15.0-156-generic', NULL, false)
	`, snapshotID, hostID, []byte{})
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES ('USN-7010-1', 'ubuntu_usn_api', '7010-1', 'ubuntu', 'security', 'high', 'Severity fallback advisory', 'vendor_db', true, now())
	`)
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-7010-1",
		PackageName:    "linux-image-5.15.0-156-generic",
		InstalledNevra: utils.Some("linux-image-5.15.0-156-generic-0:5.15.0-156.166.amd64"),
		FixedNevra:     utils.Some("linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.None[string](),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ReasonText:     utils.Some("a vendor fixed package is available but not installed"),
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	var groups []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &groups))
	require.NotEmpty(t, groups)
	assert.Equal(t, "Important", groups[0]["severity_label"])
}

func TestHosts_UniqueConstraints_API(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Test unique SSH hosts
	// 1. Create first SSH host
	rec1 := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"api-ssh-host-1","hostname":"api-ssh-1.example.com","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec1.Code)

	// 2. Try to create second SSH host with same display name
	rec2 := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"api-ssh-host-1","hostname":"api-ssh-2.example.com","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "a host with this display name already exists")

	// 3. Try to create second SSH host with same pull hostname
	rec3 := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"api-ssh-host-2","hostname":"api-ssh-1.example.com","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "an SSH host with this pull hostname already exists")

	// Test unique manual hosts
	// 1. Create first manual host
	rec4 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-1","hostname":"api-manual-1.example.com"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec4.Code)

	// 2. Try to create second manual host with same display name
	rec5 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-1","hostname":"api-manual-2.example.com"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec5.Code)
	assert.Contains(t, rec5.Body.String(), "a host with this display name already exists")
}

func TestManualHosts_API(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	// Create manual host - success
	rec1 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-new","hostname":"api-manual-new.example.com"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec1.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &payload))
	hostID := payload["host_id"].(string)
	assert.NotEmpty(t, hostID)

	// Create manual host - non-admin forbidden
	rec2 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-2","hostname":"api-manual-2.example.com"}`,
		apitesting.WithBearerToken(userToken),
	)
	require.Equal(t, http.StatusForbidden, rec2.Code)

	// Create manual host - bad JSON
	rec3 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec3.Code)

	// Create manual host - validation failure
	rec4 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"","hostname":""}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec4.Code)

	// Ingest manual report - success
	reportContent := `_PB_METADATA_HOSTNAME=api-manual-new.example.com
_PB_METADATA_ARCH=x86_64
_PB_METADATA_OS_ID=rocky
_PB_METADATA_OS_NAME=Rocky Linux
_PB_METADATA_OS_VERSION=9.3
---UPDATES_START---
---PACKAGES_START---
---REPOS_START---`

	rec5 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		reportContent,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, rec5.Code)

	// Ingest manual report - non-admin forbidden
	rec6 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		reportContent,
		apitesting.WithBearerToken(userToken),
	)
	require.Equal(t, http.StatusForbidden, rec6.Code)

	// Ingest manual report - missing/unknown host
	rec7 := backend.HTTPPost(
		"/api/v1/hosts/h_unknown123/report",
		reportContent,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec7.Code)

	// Ingest manual report - invalid payload
	rec8 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		`INVALID_REPORT_DATA`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, rec8.Code)

	// resulting snapshot/decision effects
	snapshotRecorder := backend.HTTPGet(fmt.Sprintf("/api/v1/hosts/%s/snapshot", hostID), apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, snapshotRecorder.Code)
	var snapshotPayload map[string]any
	require.NoError(t, json.Unmarshal(snapshotRecorder.Body.Bytes(), &snapshotPayload))
	assert.Equal(t, hostID, snapshotPayload["host_id"])
}
