// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/services"
	apitesting "go.patchbase.net/server/internal/testing"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
