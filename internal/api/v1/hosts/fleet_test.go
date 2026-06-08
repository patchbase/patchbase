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
	apitesting "go.patchbase.net/server/internal/testing"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
