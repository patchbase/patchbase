package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

	// Test onboarding endpoint
	onboardRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/onboard-ssh", hostID),
		`{}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusNoContent, onboardRecorder.Code)
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
