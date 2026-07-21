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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestRegistrationTokenLifecycle(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"reg-token-1"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &created))
	tokenID := created["id"].(string)
	require.NotEmpty(t, created["token"])

	listRecorder := backend.HTTPGet("/api/v1/hosts/tokens", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, listRecorder.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.NotEmpty(t, listed)
	assert.Equal(t, tokenID, listed[0]["id"])

	revokeRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/tokens/%s/revoke", tokenID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	assert.Equal(t, http.StatusOK, revokeRecorder.Code)
}

func TestRegistrationTokenValidationAndErrors(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	t.Run("anonymous create returns 401", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/hosts/tokens", `{"name":"test"}`)
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("anonymous list returns 401", func(t *testing.T) {
		recorder := backend.HTTPGet("/api/v1/hosts/tokens")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("anonymous revoke returns 401", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/hosts/tokens/rtok_test/revoke", "{}")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("non-admin create returns 403", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/tokens",
			`{"name":"test"}`,
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("non-admin list returns 403", func(t *testing.T) {
		recorder := backend.HTTPGet(
			"/api/v1/hosts/tokens",
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("non-admin revoke returns 403", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/tokens/rtok_test/revoke",
			"{}",
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("create with empty name succeeds with default", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/tokens",
			`{"name":""}`,
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusCreated, recorder.Code)
		var result map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
		assert.Equal(t, "Registration token", result["name"])
	})

	createRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"validation-token"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createRecorder.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &created))
	tokenID := created["id"].(string)
	require.NotEmpty(t, tokenID)

	t.Run("revoke unknown token returns 404", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/tokens/rtok_nonexistent/revoke",
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	revokeRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/tokens/%s/revoke", tokenID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, revokeRecorder.Code)

	t.Run("double revoke returns 404", func(t *testing.T) {
		recorder := backend.HTTPPost(
			fmt.Sprintf("/api/v1/hosts/tokens/%s/revoke", tokenID),
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("list returns tokens in created_at DESC id DESC order", func(t *testing.T) {
		tok1 := backend.HTTPPost(
			"/api/v1/hosts/tokens",
			`{"name":"order-token-1"}`,
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusCreated, tok1.Code)
		var t1 map[string]any
		require.NoError(t, json.Unmarshal(tok1.Body.Bytes(), &t1))

		// Ensure distinct created_at timestamps so the ORDER BY created_at DESC is deterministic.
		time.Sleep(2 * time.Millisecond)

		tok2 := backend.HTTPPost(
			"/api/v1/hosts/tokens",
			`{"name":"order-token-2"}`,
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusCreated, tok2.Code)
		var t2 map[string]any
		require.NoError(t, json.Unmarshal(tok2.Body.Bytes(), &t2))

		recorder := backend.HTTPGet("/api/v1/hosts/tokens", apitesting.WithBearerToken(adminToken))
		require.Equal(t, http.StatusOK, recorder.Code)
		var listed []map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &listed))
		require.GreaterOrEqual(t, len(listed), 3)

		assert.Equal(t, t2["id"], listed[0]["id"])
		assert.Equal(t, t1["id"], listed[1]["id"])
	})
}
