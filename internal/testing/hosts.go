// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package testing

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type HostRegistrationResult struct {
	HostID          string
	HostAccessToken string
}

// RegisterAndIngestHost registers a host via the agent API, approves it,
// and ingests a single snapshot so the host has OS metadata available.
func RegisterAndIngestHost(
	t *testing.T, backend *Backend, adminToken, registrationToken, hostname, osName, osVersion string,
	osMajor int32, osFamily agentpb.OsFamily, architecture agentpb.Architecture,
) HostRegistrationResult {
	t.Helper()

	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: registrationToken,
		Hostname:          hostname,
		MachineId:         hostname + "-machine",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.50",
			OsName:       osName,
			OsVersion:    osVersion,
			Architecture: "x86_64",
		},
	}
	registerReqBytes, err := proto.Marshal(registerReq)
	require.NoError(t, err)

	registerRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/register",
		registerReqBytes,
		WithHeader("Content-Type", "application/x-protobuf"),
	)
	require.Equal(t, http.StatusCreated, registerRecorder.Code)

	var registered agentpb.RegisterHostResponse
	require.NoError(t, proto.Unmarshal(registerRecorder.Body.Bytes(), &registered))

	approveRecorder := backend.HTTPPost(
		"/api/v1/hosts/"+registered.HostId+"/approve",
		"{}",
		WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder.Code)

	snapshot := &agentpb.AgentSnapshot{ // nolint: exhaustruct
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{ // nolint: exhaustruct
			MachineId:    hostname + "-machine",
			Hostname:     hostname,
			OsFamily:     osFamily,
			OsName:       osName,
			OsMajor:      osMajor,
			OsVersion:    osVersion,
			Architecture: architecture,
			IpAddresses:  []string{"10.0.0.50"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-5.14.0"}, // nolint: exhaustruct
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	ingestRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		WithHeader("Content-Type", "application/x-protobuf"),
		WithBearerToken(registered.HostAccessToken),
	)
	require.Equal(t, http.StatusAccepted, ingestRecorder.Code)

	return HostRegistrationResult{
		HostID:          registered.HostId,
		HostAccessToken: registered.HostAccessToken,
	}
}
