// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
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

func TestRegisterNegativePaths(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"token-neg"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)

	var createTokenPayload map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &createTokenPayload))
	validRegToken := createTokenPayload["token"].(string)
	tokenID := createTokenPayload["id"].(string)

	baseReq := &agentpb.RegisterHostRequest{
		RegistrationToken: validRegToken,
		Hostname:          "agent-neg-01",
		MachineId:         "machine-neg-001",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.12",
			OsName:       "Rocky Linux",
			OsVersion:    "9.5",
			Architecture: "x86_64",
		},
	}

	t.Run("invalid protobuf", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			[]byte("invalid-proto-bytes"),
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("JSON body content-type mismatch", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			[]byte(`{"registration_token":"`+validRegToken+`"}`),
			apitesting.WithHeader("Content-Type", "application/json"),
		)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("missing registration token", func(t *testing.T) {
		req := proto.Clone(baseReq).(*agentpb.RegisterHostRequest)
		req.RegistrationToken = ""
		reqBytes, _ := proto.Marshal(req)
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("invalid registration token", func(t *testing.T) {
		req := proto.Clone(baseReq).(*agentpb.RegisterHostRequest)
		req.RegistrationToken = "invalid-token"
		reqBytes, _ := proto.Marshal(req)
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("malformed metadata", func(t *testing.T) {
		req := proto.Clone(baseReq).(*agentpb.RegisterHostRequest)
		req.Metadata = nil
		reqBytes, _ := proto.Marshal(req)
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("duplicate machine ID behavior", func(t *testing.T) {
		reqBytes, _ := proto.Marshal(baseReq)
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusCreated, recorder.Code)

		// Register again with the same machine ID and hostname
		recorderDup := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusConflict, recorderDup.Code)

		// Registering with an existing machine ID but a new hostname should succeed.
		reqDiffHostname := proto.Clone(baseReq).(*agentpb.RegisterHostRequest)
		reqDiffHostname.Hostname = "agent-neg-02"
		reqDiffHostnameBytes, _ := proto.Marshal(reqDiffHostname)
		recorderDupDiff := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqDiffHostnameBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusCreated, recorderDupDiff.Code)
	})

	t.Run("revoked token", func(t *testing.T) {
		revokeRecorder := backend.HTTPPost(
			fmt.Sprintf("/api/v1/hosts/tokens/%s/revoke", tokenID),
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusOK, revokeRecorder.Code)

		// Use a different hostname to ensure we are testing the revoked token behavior,
		// rather than failing due to the hostname unique constraint violation.
		req := proto.Clone(baseReq).(*agentpb.RegisterHostRequest)
		req.Hostname = "agent-neg-03"
		reqBytesDiff, _ := proto.Marshal(req)

		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/register",
			reqBytesDiff,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}

func TestSnapshotNegativePaths(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"token-snap-neg"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)

	var createTokenPayload map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &createTokenPayload))
	validRegToken := createTokenPayload["token"].(string)

	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: validRegToken,
		Hostname:          "agent-snap-neg-01",
		MachineId:         "machine-snap-neg-001",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.12",
			OsName:       "Rocky Linux",
			OsVersion:    "9.5",
			Architecture: "x86_64",
		},
	}
	registerReqBytes, _ := proto.Marshal(registerReq)

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

	// Create another host
	registerReq2 := &agentpb.RegisterHostRequest{
		RegistrationToken: validRegToken,
		Hostname:          "agent-snap-neg-02",
		MachineId:         "machine-snap-neg-002",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.13",
			OsName:       "Rocky Linux",
			OsVersion:    "9.5",
			Architecture: "x86_64",
		},
	}
	registerReqBytes2, _ := proto.Marshal(registerReq2)
	registerRecorder2 := backend.HTTPPostBytes(
		"/api/v1/agent/register",
		registerReqBytes2,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
	)
	require.Equal(t, http.StatusCreated, registerRecorder2.Code)
	var registerPayload2 agentpb.RegisterHostResponse
	require.NoError(t, proto.Unmarshal(registerRecorder2.Body.Bytes(), &registerPayload2))
	hostID2 := registerPayload2.HostId
	hostAccessToken2 := registerPayload2.HostAccessToken

	// Approve both hosts
	approveRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder.Code)

	approveRecorder2 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID2),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder2.Code)

	baseSnapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "machine-snap-neg-001",
			Hostname:                    "agent-snap-neg-01",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:                      "Rocky Linux",
			OsMajor:                     9,
			OsVersion:                   "9.5",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 7,
			IpAddresses:                 []string{"10.0.0.12"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-5.14.0"},
	}
	payloadBytes, _ := proto.Marshal(baseSnapshot)

	t.Run("missing bearer token", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			payloadBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("invalid bearer token", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			payloadBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
			apitesting.WithBearerToken("invalid-token"),
		)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("malformed protobuf", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			[]byte("invalid-proto"),
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
			apitesting.WithBearerToken(hostAccessToken),
		)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("validation failures", func(t *testing.T) {
		emptySnap := &agentpb.AgentSnapshot{}
		emptyBytes, _ := proto.Marshal(emptySnap)
		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			emptyBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
			apitesting.WithBearerToken(hostAccessToken),
		)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("host token for another host", func(t *testing.T) {
		snapHost2 := proto.Clone(baseSnapshot).(*agentpb.AgentSnapshot)
		snapHost2.Host.MachineId = "machine-snap-neg-002"
		snapHost2.Host.Hostname = "agent-snap-neg-02"
		snapHost2Bytes, _ := proto.Marshal(snapHost2)

		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			snapHost2Bytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
			apitesting.WithBearerToken(hostAccessToken),
		)
		// Expected to be rejected because machine ID doesn't match the token's host.
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("matcher failure response behavior", func(t *testing.T) {
		snapFail := proto.Clone(baseSnapshot).(*agentpb.AgentSnapshot)
		snapFail.Host.MachineId = "machine-snap-neg-002"
		snapFail.Host.OsFamily = agentpb.OsFamily_OS_FAMILY_UNSPECIFIED
		snapFail.Host.OsName = ""
		snapFailBytes, _ := proto.Marshal(snapFail)

		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			snapFailBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
			apitesting.WithBearerToken(hostAccessToken2),
		)
		// Should still return 202 Accepted even if matcher/advisory-sync fails
		assert.Equal(t, http.StatusAccepted, recorder.Code)
	})

	t.Run("deleted host", func(t *testing.T) {
		deleteRecorder := backend.HTTPDelete(fmt.Sprintf("/api/v1/hosts/%s", hostID), apitesting.WithBearerToken(adminToken))
		require.Equal(t, http.StatusOK, deleteRecorder.Code)

		recorder := backend.HTTPPostBytes(
			"/api/v1/agent/snapshots",
			payloadBytes,
			apitesting.WithHeader("Content-Type", "application/x-protobuf"),
			apitesting.WithBearerToken(hostAccessToken),
		)
		// Expected to be unauthorized so agent is forced to re-register
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}
