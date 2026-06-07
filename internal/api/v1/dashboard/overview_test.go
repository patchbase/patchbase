package dashboard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestDashboardOverviewEndpoint(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Fetch dashboard overview when database is empty
	recorder := backend.HTTPGet("/api/v1/dashboard/overview", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var overview map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &overview))

	// Verify default counts when empty
	assert.Equal(t, float64(0), overview["total_hosts"])
	assert.Equal(t, float64(0), overview["need_attention"])
	assert.Equal(t, float64(0), overview["reboot_queue"])
	assert.Equal(t, float64(0), overview["unknown_investigate"])
	assert.Equal(t, float64(0), overview["total_advisories"])
	assert.Equal(t, float64(0), overview["total_scopes"])
}
