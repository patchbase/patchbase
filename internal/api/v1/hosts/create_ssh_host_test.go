// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/services"
	apitesting "go.patchbase.net/server/internal/testing"
)

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

func TestCreateSSHHostValidation(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	tests := []struct {
		name           string
		body           string
		token          string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing display name",
			body:           `{"hostname":"203.0.113.10","ssh_user":"root","frequency_minutes":60}`,
			token:          adminToken,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "display name is required",
		},
		{
			name:           "missing hostname",
			body:           `{"display_name":"test","ssh_user":"root","frequency_minutes":60}`,
			token:          adminToken,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "hostname is required",
		},
		{
			name:           "invalid frequency",
			body:           `{"display_name":"test","hostname":"203.0.113.10","ssh_user":"root","frequency_minutes":-5}`,
			token:          adminToken,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid frequency",
		},
		{
			name:           "non-admin access",
			body:           `{"display_name":"test","hostname":"203.0.113.10","ssh_user":"root","frequency_minutes":60}`,
			token:          userToken,
			expectedStatus: http.StatusForbidden,
			expectedError:  "only admins can create ssh hosts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := backend.HTTPPost(
				"/api/v1/hosts/ssh",
				tt.body,
				apitesting.WithBearerToken(tt.token),
			)
			require.Equal(t, tt.expectedStatus, recorder.Code)
			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}
		})
	}

	// Test global-default SSH user behavior
	settingsService := do.MustInvoke[services.Settings](backend.Injector())
	err = settingsService.SetDefaultSSHPullUser(context.Background(), services.SystemActorRef(), "default-admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"test-default-user","hostname":"203.0.113.15","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, recorder.Code)
}

func TestCreateSSHHostValidation_NoDefaultUser(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			originalSettings := do.MustInvoke[services.Settings](i)
			do.Override[services.Settings](i, func(i do.Injector) (services.Settings, error) {
				return &mockSettingsNoDefault{Settings: originalSettings}, nil
			})
		}),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"test","hostname":"203.0.113.10","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "ssh user is required")
}

func TestCreateSSHHostValidation_InternalError(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			originalSettings := do.MustInvoke[services.Settings](i)
			do.Override[services.Settings](i, func(i do.Injector) (services.Settings, error) {
				return &mockSettingsInternalError{Settings: originalSettings}, nil
			})
		}),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"test","hostname":"203.0.113.10","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "internal server error")
}

type mockSettingsNoDefault struct {
	services.Settings
}

func (m *mockSettingsNoDefault) GetDefaultSSHPullUser(ctx context.Context) (string, error) {
	return "", nil
}

type mockSettingsInternalError struct {
	services.Settings
}

func (m *mockSettingsInternalError) GetDefaultSSHPullUser(ctx context.Context) (string, error) {
	return "", fmt.Errorf("database down")
}
