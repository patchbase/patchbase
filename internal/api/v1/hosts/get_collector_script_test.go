// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestGetCollectorScriptIncludesContentLength(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		"/api/v1/hosts/manual/script?os_family=apt",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	assert.Equal(t, `attachment; filename="patchbase-collector.sh"`, recorder.Header().Get("Content-Disposition"))
	assert.Equal(t, "text/x-shellscript", recorder.Header().Get("Content-Type"))
	assert.Equal(t, strconv.Itoa(recorder.Body.Len()), recorder.Header().Get("Content-Length"))
	assert.NotEmpty(t, recorder.Body.String())
}

func TestGetCollectorScriptRequiresOSFamily(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		"/api/v1/hosts/manual/script",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "missing required query parameter: os_family")
}

func TestGetCollectorScriptRejectsUnsupportedOSFamily(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		"/api/v1/hosts/manual/script?os_family=solaris",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unsupported os family")
}
