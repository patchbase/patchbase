package settings_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestPatchSettings(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	// 1. Verify anonymous access is denied
	recorderAnon := backend.HTTPPatch("/api/v1/settings", `{"default_ssh_pull_user": "admin"}`)
	assert.Equal(t, http.StatusUnauthorized, recorderAnon.Code)

	// 2. Verify non-admin access is forbidden
	recorderUser := backend.HTTPPatch("/api/v1/settings", `{"default_ssh_pull_user": "admin"}`, apitesting.WithBearerToken(userToken))
	assert.Equal(t, http.StatusForbidden, recorderUser.Code)

	// 3. Verify validation fails for empty user
	recorderEmpty := backend.HTTPPatch("/api/v1/settings", `{"default_ssh_pull_user": "   "}`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusBadRequest, recorderEmpty.Code)

	// 4. Verify successful update
	recorderSuccess := backend.HTTPPatch("/api/v1/settings", `{"default_ssh_pull_user": "ubuntu"}`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusOK, recorderSuccess.Code)

	// 5. Verify the setting was actually updated
	recorderGet := backend.HTTPGet("/api/v1/settings", apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusOK, recorderGet.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorderGet.Body.Bytes(), &payload))
	assert.Equal(t, "ubuntu", payload["default_ssh_pull_user"])
}

func TestPatchSettingsMalformedJSON(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPatch("/api/v1/settings", `not json`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"error":"invalid request body"`)
}

func TestPatchSettingsValidation(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	t.Run("default ssh pull user with only whitespace", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/settings", `{"default_ssh_pull_user": "   "}`, apitesting.WithBearerToken(adminToken))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.JSONEq(t, `{"error":"default ssh pull user cannot be empty"}`, recorder.Body.String())
	})

	t.Run("default ssh pull user with empty string", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/settings", `{"default_ssh_pull_user": ""}`, apitesting.WithBearerToken(adminToken))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.JSONEq(t, `{"error":"default ssh pull user cannot be empty"}`, recorder.Body.String())
	})

	t.Run("ask to copy public key with true", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/settings", `{"ask_to_copy_public_key": true}`, apitesting.WithBearerToken(adminToken))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("ask to copy public key with false", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/settings", `{"ask_to_copy_public_key": false}`, apitesting.WithBearerToken(adminToken))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("empty body", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/settings", `{}`, apitesting.WithBearerToken(adminToken))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}
