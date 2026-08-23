// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package lifecycle_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestLifecycleCatalogSourceEndpoint(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet("/api/v1/lifecycle/catalog", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var source map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &source))
	assert.NotEmpty(t, source["generated_at"])
	assert.NotEmpty(t, source["source_urls"])
}

func TestLifecycleEndpointsRequireAuth(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)

	listRecorder := backend.HTTPGet("/api/v1/lifecycle/hosts")
	require.Equal(t, http.StatusUnauthorized, listRecorder.Code)

	catalogRecorder := backend.HTTPGet("/api/v1/lifecycle/catalog")
	require.Equal(t, http.StatusUnauthorized, catalogRecorder.Code)
}