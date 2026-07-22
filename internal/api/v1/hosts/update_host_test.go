// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestUpdateHost(t *testing.T) {
	backend := apitesting.NewBackend(t, apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")))
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	created := backend.HTTPPost("/api/v1/hosts/ssh", `{"display_name":"old-name","hostname":"old.example","ssh_user":"root","frequency_minutes":60}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusCreated, created.Code)
	var createdPayload map[string]any
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdPayload))
	hostID := createdPayload["host_id"].(string)
	path := fmt.Sprintf("/api/v1/hosts/%s", hostID)

	updated := backend.HTTPPatch(path, `{"display_name":" new-name ","pull_hostname":"new.example","pull_ssh_user":"patchbase","pull_frequency_minutes":30}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	var payload struct {
		DisplayName   string `json:"display_name"`
		Configuration struct {
			Hostname  string `json:"pull_hostname"`
			SSHUser   string `json:"pull_ssh_user"`
			Frequency int32  `json:"pull_frequency_minutes"`
		} `json:"configuration"`
	}
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &payload))
	assert.Equal(t, "new-name", payload.DisplayName)
	assert.Equal(t, "new.example", payload.Configuration.Hostname)
	assert.Equal(t, "patchbase", payload.Configuration.SSHUser)
	assert.Equal(t, int32(30), payload.Configuration.Frequency)

	tests := []struct {
		name   string
		body   string
		token  string
		status int
	}{
		{name: "immutable field", body: `{"onboarding_mode":"manual"}`, token: adminToken, status: http.StatusBadRequest},
		{name: "approval status", body: `{"approval_status":"rejected"}`, token: adminToken, status: http.StatusBadRequest},
		{name: "frequency too low", body: `{"pull_frequency_minutes":4}`, token: adminToken, status: http.StatusBadRequest},
		{name: "empty display name", body: `{"display_name":" "}`, token: adminToken, status: http.StatusBadRequest},
		{name: "non admin", body: `{"display_name":"forbidden"}`, token: userToken, status: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := backend.HTTPPatch(path, tt.body, apitesting.WithBearerToken(tt.token))
			assert.Equal(t, tt.status, response.Code, response.Body.String())
		})
	}
}

func TestUpdateHostRejectsSSHFieldsForManualHost(t *testing.T) {
	backend := apitesting.NewBackend(t, apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")))
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	created := backend.HTTPPost("/api/v1/hosts/manual", `{"display_name":"manual","hostname":"manual.example"}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusCreated, created.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &payload))

	response := backend.HTTPPatch(fmt.Sprintf("/api/v1/hosts/%s", payload["host_id"]), `{"pull_ssh_user":"root"}`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
}
