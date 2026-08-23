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
	agentpb "go.patchbase.net/proto/agent"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestLifecycleListHostsEndpoint(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	registrationToken := apitesting.CreateRegistrationToken(t, backend, adminToken, "lifecycle-token")
	host := apitesting.RegisterAndIngestHost(
		t, backend, adminToken, registrationToken, "rocky-host", "Rocky Linux", "9.5", 9,
		agentpb.OsFamily_OS_FAMILY_RPM, agentpb.Architecture_ARCHITECTURE_X86_64,
	)

	recorder := backend.HTTPGet("/api/v1/lifecycle/hosts", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var hosts []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &hosts))
	require.NotEmpty(t, hosts)

	var found map[string]any
	for _, h := range hosts {
		if h["id"] == host.HostID {
			found = h
			break
		}
	}
	require.NotNil(t, found, "registered host not found in lifecycle list")

	lc, ok := found["lifecycle"].(map[string]any)
	require.True(t, ok, "lifecycle block missing on host")
	assert.Equal(t, "supported", lc["state"])
	assert.Equal(t, "rocky", lc["product_key"])
	assert.Equal(t, "Rocky Linux", lc["product_display_name"])
	assert.Equal(t, "9", lc["cycle"])
	assert.NotEmpty(t, lc["standard_support_end"])
	assert.NotEmpty(t, lc["source_url"])
}

func TestLifecycleListHostsUnknownOS(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	registrationToken := apitesting.CreateRegistrationToken(t, backend, adminToken, "lifecycle-token")
	host := apitesting.RegisterAndIngestHost(
		t, backend, adminToken, registrationToken, "solaris-host", "OpenSolaris", "11", 11,
		agentpb.OsFamily_OS_FAMILY_UNSPECIFIED, agentpb.Architecture_ARCHITECTURE_X86_64,
	)

	recorder := backend.HTTPGet("/api/v1/lifecycle/hosts", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var hosts []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &hosts))

	var found map[string]any
	for _, h := range hosts {
		if h["id"] == host.HostID {
			found = h
			break
		}
	}
	require.NotNil(t, found)

	lc, ok := found["lifecycle"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "unknown", lc["state"])
	assert.Empty(t, lc["product_key"])
}