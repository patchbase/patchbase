package profile_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestGetProfile(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorderAnon := backend.HTTPGet("/api/v1/profile")
	assert.Equal(t, http.StatusUnauthorized, recorderAnon.Code)

	recorder := backend.HTTPGet("/api/v1/profile", apitesting.WithBearerToken(token))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, token, payload["access_token"])

	user, ok := payload["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "u_admin", user["id"])
	assert.Equal(t, "admin@patchbase.local", user["email"])
	assert.Equal(t, "Admin", user["name"])
}

func TestUpdateProfileEmail(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPatch("/api/v1/profile", `{"email":" Owner@PatchBase.Local "}`, apitesting.WithBearerToken(token))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, token, payload["access_token"])

	user, ok := payload["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "owner@patchbase.local", user["email"])

	loginRecorder := backend.HTTPPost("/api/v1/auth/login", `{
		"email":"owner@patchbase.local",
		"password":"password"
	}`)
	assert.Equal(t, http.StatusOK, loginRecorder.Code)
}

func TestUpdateProfileEmailDuplicate(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPatch("/api/v1/profile", `{"email":"user@patchbase.local"}`, apitesting.WithBearerToken(token))

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.JSONEq(t, `{"code":"email_already_in_use","message":"email is already in use"}`, recorder.Body.String())
}

func TestUpdateProfilePasswordRequiresCurrentPassword(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorderMissing := backend.HTTPPatch("/api/v1/profile", `{"new_password":"new-secure-password"}`, apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusBadRequest, recorderMissing.Code)
	assert.JSONEq(t, `{"code":"current_password_required","message":"current password is required"}`, recorderMissing.Body.String())

	recorderWrong := backend.HTTPPatch(
		"/api/v1/profile",
		`{"current_password":"wrong-password","new_password":"new-secure-password"}`,
		apitesting.WithBearerToken(token),
	)
	assert.Equal(t, http.StatusUnauthorized, recorderWrong.Code)
	assert.JSONEq(t, `{"code":"current_password_invalid","message":"current password is invalid"}`, recorderWrong.Body.String())
}

func TestUpdateProfilePasswordReturnsFreshToken(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPatch(
		"/api/v1/profile",
		`{"current_password":"password","new_password":"new-secure-password"}`,
		apitesting.WithBearerToken(token),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	newToken, ok := payload["access_token"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, token, newToken)

	oldTokenRecorder := backend.HTTPGet("/api/v1/profile", apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusUnauthorized, oldTokenRecorder.Code)

	newTokenRecorder := backend.HTTPGet("/api/v1/profile", apitesting.WithBearerToken(newToken))
	assert.Equal(t, http.StatusOK, newTokenRecorder.Code)

	loginRecorder := backend.HTTPPost("/api/v1/auth/login", `{
		"email":"admin@patchbase.local",
		"password":"new-secure-password"
	}`)
	assert.Equal(t, http.StatusOK, loginRecorder.Code)
}

func TestUpdateProfileMalformedJSON(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPatch("/api/v1/profile", `{broken`, apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.JSONEq(t, `{"code":"invalid_request_body","message":"invalid request body"}`, recorder.Body.String())
}

func TestUpdateProfileEmailValidation(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	t.Run("empty email", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/profile", `{"email":""}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.JSONEq(t, `{"code":"email_required","message":"email is required"}`, recorder.Body.String())
	})

	t.Run("whitespace only email", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/profile", `{"email":"   "}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.JSONEq(t, `{"code":"email_required","message":"email is required"}`, recorder.Body.String())
	})
}

func TestUpdateProfilePasswordValidation(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	t.Run("new password too short", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/profile", `{
			"current_password":"password",
			"new_password":"short"
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.JSONEq(t, `{"code":"password_too_short","message":"password must be at least 12 characters"}`, recorder.Body.String())
	})

	t.Run("empty current password", func(t *testing.T) {
		recorder := backend.HTTPPatch("/api/v1/profile", `{
			"current_password":"",
			"new_password":"new-secure-password"
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.JSONEq(t, `{"code":"current_password_required","message":"current password is required"}`, recorder.Body.String())
	})
}

func TestUpdateProfileEmptyBody(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPatch("/api/v1/profile", `{}`, apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusOK, recorder.Code)
}
