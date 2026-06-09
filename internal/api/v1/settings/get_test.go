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

func TestGetSettings(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	// 1. Verify anonymous access is denied
	recorderAnon := backend.HTTPGet("/api/v1/settings")
	assert.Equal(t, http.StatusUnauthorized, recorderAnon.Code)

	// 2. Verify non-admin access is forbidden
	recorderUser := backend.HTTPGet("/api/v1/settings", apitesting.WithBearerToken(userToken))
	assert.Equal(t, http.StatusForbidden, recorderUser.Code)

	// 3. Verify admin access returns global public key
	recorderAdmin := backend.HTTPGet("/api/v1/settings", apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusOK, recorderAdmin.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorderAdmin.Body.Bytes(), &payload))
	assert.NotEmpty(t, payload["global_ssh_public_key"])
	assert.Contains(t, payload["global_ssh_public_key"], "ssh-ed25519")
	assert.Equal(t, "root", payload["default_ssh_pull_user"])
}
