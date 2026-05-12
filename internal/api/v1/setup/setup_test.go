package setup_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/services"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestStatusDefaultFalse(t *testing.T) {
	backend := apitesting.NewBackend(t)
	recorder := backend.HTTPGet("/api/v1/setup/status")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"completed":false}`, recorder.Body.String())
}

func TestCompleteSuccess(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost("/api/v1/setup/complete", `{
		"name":"Setup Admin",
		"email":"owner@patchbase.local",
		"password":"very-secure-pass"
	}`, apitesting.WithBearerToken(token))

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.NotEmpty(t, payload["access_token"])
	assert.Equal(t, true, payload["setup_completed"])
	assert.Equal(t, false, payload["password_reset_needed"])

	settings := do.MustInvoke[services.Settings](backend.Injector())
	status, err := settings.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, status.Done)

	loginRecorder := backend.HTTPPost("/api/v1/auth/login", `{
		"email":"owner@patchbase.local",
		"password":"very-secure-pass"
	}`)

	assert.Equal(t, http.StatusOK, loginRecorder.Code)
}
