// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

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

func TestGetHostIncludesLifecycleBlock(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	registrationToken := apitesting.CreateRegistrationToken(t, backend, adminToken, "lifecycle-host-token")
	host := apitesting.RegisterAndIngestHost(
		t, backend, adminToken, registrationToken, "rocky-lifecycle-host", "Rocky Linux", "9.5", 9,
		agentpb.OsFamily_OS_FAMILY_RPM, agentpb.Architecture_ARCHITECTURE_X86_64,
	)

	recorder := backend.HTTPGet("/api/v1/hosts/"+host.HostID, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, host.HostID, payload["id"])

	lc, ok := payload["lifecycle"].(map[string]any)
	require.True(t, ok, "lifecycle block missing on single-host response")
	assert.Equal(t, "supported", lc["state"])
	assert.Equal(t, "rocky", lc["product_key"])
	assert.Equal(t, "9", lc["cycle"])
}

func TestGetHostLifecycleUnknownOS(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	registrationToken := apitesting.CreateRegistrationToken(t, backend, adminToken, "lifecycle-unknown-token")
	host := apitesting.RegisterAndIngestHost(
		t, backend, adminToken, registrationToken, "solaris-lifecycle-host", "OpenSolaris", "11", 11,
		agentpb.OsFamily_OS_FAMILY_UNSPECIFIED, agentpb.Architecture_ARCHITECTURE_X86_64,
	)

	recorder := backend.HTTPGet("/api/v1/hosts/"+host.HostID, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	lc, ok := payload["lifecycle"].(map[string]any)
	require.True(t, ok, "lifecycle block missing on single-host response")
	assert.Equal(t, "unknown", lc["state"])
	assert.Empty(t, lc["product_key"])
}
