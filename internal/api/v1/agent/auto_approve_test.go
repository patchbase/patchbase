// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	db "go.patchbase.net/server/internal/sql"
	apitesting "go.patchbase.net/server/internal/testing"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAutoApproveRegistrationFlow(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Create a registration token with auto_approve enabled.
	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"auto-approve-token","auto_approve":true}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)
	var createTokenPayload map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &createTokenPayload))
	registrationToken := createTokenPayload["token"].(string)
	require.NotEmpty(t, registrationToken)

	// The token list must preserve the auto_approve flag.
	listRecorder := backend.HTTPGet("/api/v1/hosts/tokens", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, listRecorder.Code)
	var listed []map[string]any
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.NotEmpty(t, listed)
	assert.Equal(t, true, listed[0]["auto_approve"])

	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: registrationToken,
		Hostname:          "auto-approved-host",
		MachineId:         "auto-approved-machine",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.77",
			OsName:       "Ubuntu",
			OsVersion:    "24.04",
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
	require.NotEmpty(t, hostID)
	require.NotEmpty(t, hostAccessToken)
	assert.Equal(t, "approved", registerPayload.ApprovalStatus)

	// The host must not appear in the pending-approvals list.
	pendingRecorder := backend.HTTPGet("/api/v1/hosts/pending", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, pendingRecorder.Code)
	var pendingPayload []map[string]any
	require.NoError(t, json.Unmarshal(pendingRecorder.Body.Bytes(), &pendingPayload))
	for _, h := range pendingPayload {
		assert.NotEqual(t, hostID, h["id"], "auto-approved host should not be pending")
	}

	// approved_at must be populated in the database.
	queries := db.New(backend.DB())
	hostRow, err := queries.GetHostByID(context.Background(), hostID)
	require.NoError(t, err)
	assert.True(t, hostRow.ApprovedAt.Valid, "approved_at should be set")
	assert.WithinDuration(t, time.Now().UTC(), hostRow.ApprovedAt.Time.UTC(), 5*time.Second)

	// A snapshot must be accepted without manual approval.
	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "auto-approved-machine",
			Hostname:                    "auto-approved-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_APT,
			OsName:                      "Ubuntu",
			OsMajor:                     24,
			OsVersion:                   "24.04",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 3,
			IpAddresses:                 []string{"10.0.0.77"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-6.8.0"},
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
	assert.Equal(t, hostID, acceptedPayload.HostId)
	assert.NotEmpty(t, acceptedPayload.SnapshotId)
}
