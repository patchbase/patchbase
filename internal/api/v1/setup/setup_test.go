// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
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

func TestCompleteMalformedJSON(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost("/api/v1/setup/complete", `{invalid}`, apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.JSONEq(t, `{"code":"invalid_request_body","message":"invalid request body"}`, recorder.Body.String())
}

func TestCompleteValidationErrors(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	t.Run("empty name", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{
			"name":"",
			"email":"owner@patchbase.local",
			"password":"very-secure-pass"
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("empty email", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{
			"name":"Admin",
			"email":"",
			"password":"very-secure-pass"
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("whitespace only email", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{
			"name":"Admin",
			"email":"   ",
			"password":"very-secure-pass"
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("password too short", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{
			"name":"Admin",
			"email":"owner@patchbase.local",
			"password":"short"
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("all fields empty", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{
			"name":"",
			"email":"",
			"password":""
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("missing fields", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("nil value fields", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/setup/complete", `{
			"name":null,
			"email":null,
			"password":null
		}`, apitesting.WithBearerToken(token))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestCompleteEmailAlreadyInUse(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPPost("/api/v1/setup/complete", `{
		"name":"Admin",
		"email":"user@patchbase.local",
		"password":"very-secure-pass"
	}`, apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.JSONEq(t, `{"code":"email_already_in_use","message":"email is already in use"}`, recorder.Body.String())
}

func TestCompleteAlreadyCompleted(t *testing.T) {
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
	newToken, ok := payload["access_token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, newToken)

	recorderDup := backend.HTTPPost("/api/v1/setup/complete", `{
		"name":"Another Admin",
		"email":"other@patchbase.local",
		"password":"very-secure-pass"
	}`, apitesting.WithBearerToken(newToken))
	assert.Equal(t, http.StatusConflict, recorderDup.Code)
	assert.JSONEq(t, `{"code":"initial_setup_already_complete","message":"initial setup already completed"}`, recorderDup.Body.String())
}

func TestCompleteUnauthorized(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)

	recorder := backend.HTTPPost("/api/v1/setup/complete", `{
		"name":"Admin",
		"email":"owner@patchbase.local",
		"password":"very-secure-pass"
	}`)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestCompleteForbiddenNonAdmin(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	token, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	recorder := backend.HTTPPost("/api/v1/setup/complete", `{
		"name":"User Admin",
		"email":"user-admin@patchbase.local",
		"password":"very-secure-pass"
	}`, apitesting.WithBearerToken(token))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.JSONEq(t, `{"code":"forbidden_complete_setup","message":"only admins can complete setup"}`, recorder.Body.String())
}
