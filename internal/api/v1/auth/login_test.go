package auth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestLoginSuccess(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	recorder := backend.HTTPPost("/api/v1/auth/login", `{
		"email":"admin@patchbase.local",
		"password":"password"
	}`)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	assert.NotEmpty(t, payload["access_token"])
	assert.Equal(t, false, payload["setup_completed"])
	assert.Equal(t, false, payload["password_reset_needed"])

	user, ok := payload["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "u_admin", user["id"])
	assert.Equal(t, "admin@patchbase.local", user["email"])
	assert.Equal(t, "Admin", user["name"])
}

func TestLoginInvalidCredentials(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	recorder := backend.HTTPPost("/api/v1/auth/login", `{
		"email":"admin@patchbase.local",
		"password":"wrong-password"
	}`)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.JSONEq(t, `{"code":"invalid_credentials","message":"invalid email or password"}`, recorder.Body.String())
}
