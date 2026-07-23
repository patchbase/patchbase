// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestUpdateHostNotes(t *testing.T) {
	backend := apitesting.NewBackend(t, apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")))
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	created := backend.HTTPPost("/api/v1/hosts/manual", `{"display_name":"notes-host","hostname":"notes.example"}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusCreated, created.Code)
	var createdPayload map[string]any
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdPayload))
	hostID := createdPayload["host_id"].(string)
	path := fmt.Sprintf("/api/v1/hosts/%s/notes", hostID)

	updated := backend.HTTPPut(path, `{"notes":"First line\nSecond line   "}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	assert.JSONEq(t, `"First line\nSecond line"`, extractJSONField(t, updated.Body.Bytes(), "notes"))

	getResponse := backend.HTTPGet(fmt.Sprintf("/api/v1/hosts/%s", hostID), apitesting.WithBearerToken(userToken))
	require.Equal(t, http.StatusOK, getResponse.Code)
	assert.JSONEq(t, `"First line\nSecond line"`, extractJSONField(t, getResponse.Body.Bytes(), "notes"))

	forbidden := backend.HTTPPut(path, `{"notes":"forbidden"}`, apitesting.WithBearerToken(userToken))
	assert.Equal(t, http.StatusForbidden, forbidden.Code)

	tooLargeBody, err := json.Marshal(map[string]string{"notes": strings.Repeat("a", 8193)})
	require.NoError(t, err)
	tooLarge := backend.HTTPPut(path, string(tooLargeBody), apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusRequestEntityTooLarge, tooLarge.Code)

	oversizedBody, err := json.Marshal(map[string]string{"notes": strings.Repeat("a", 129*1024)})
	require.NoError(t, err)
	oversized := backend.HTTPPut(path, string(oversizedBody), apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusRequestEntityTooLarge, oversized.Code)

	unknownField := backend.HTTPPut(path, `{"notes":"valid","other":true}`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusBadRequest, unknownField.Code)

	missingNotes := backend.HTTPPut(path, `{}`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusBadRequest, missingNotes.Code)

	missingHost := backend.HTTPPut("/api/v1/hosts/h_missing/notes", `{"notes":"note"}`, apitesting.WithBearerToken(adminToken))
	assert.Equal(t, http.StatusNotFound, missingHost.Code)

	cleared := backend.HTTPPut(path, `{"notes":""}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, cleared.Code)
	assert.JSONEq(t, `null`, extractJSONField(t, cleared.Body.Bytes(), "notes"))

	clearedNull := backend.HTTPPut(path, `{"notes":null}`, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, clearedNull.Code)
	assert.JSONEq(t, `null`, extractJSONField(t, clearedNull.Body.Bytes(), "notes"))
}

func extractJSONField(t *testing.T, payload []byte, field string) string {
	t.Helper()
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &object))
	value, ok := object[field]
	require.True(t, ok)
	return string(value)
}
