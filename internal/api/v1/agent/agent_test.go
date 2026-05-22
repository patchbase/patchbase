package agent_test

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

func TestRegisterAndSnapshotFlow(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"token-a"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)

	var createTokenPayload map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &createTokenPayload))
	registrationToken := createTokenPayload["token"].(string)
	require.NotEmpty(t, registrationToken)

	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: registrationToken,
		Hostname:          "agent-host-01",
		MachineId:         "machine-001",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.11",
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

	var registerPayload agentpb.RegisterHostResponse
	require.NoError(t, proto.Unmarshal(registerRecorder.Body.Bytes(), &registerPayload))
	hostID := registerPayload.HostId
	hostAccessToken := registerPayload.HostAccessToken
	assert.Equal(t, "waiting_approval", registerPayload.ApprovalStatus)
	require.NotEmpty(t, hostID)
	require.NotEmpty(t, hostAccessToken)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "machine-001",
			Hostname:                    "agent-host-01",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:                      "Rocky Linux",
			OsMajor:                     9,
			OsVersion:                   "9.5",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 7,
			IpAddresses:                 []string{"10.0.0.11"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-5.14.0"},
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	pendingRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostAccessToken),
	)
	assert.Equal(t, http.StatusForbidden, pendingRecorder.Code)

	approveRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder.Code)

	acceptedRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostAccessToken),
	)
	require.Equal(t, http.StatusAccepted, acceptedRecorder.Code)

	var acceptedPayload agentpb.SyncResponse
	require.NoError(t, proto.Unmarshal(acceptedRecorder.Body.Bytes(), &acceptedPayload))
	assert.Equal(t, true, acceptedPayload.Accepted)
	assert.Equal(t, hostID, acceptedPayload.HostId)
	assert.NotEmpty(t, acceptedPayload.SnapshotId)
}
