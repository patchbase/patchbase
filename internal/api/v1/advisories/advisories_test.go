package advisories_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/services"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestAdvisoryEndpoints(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Inject and register a scope to test with
	advisorySyncService := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	ctx := context.Background()
	err = advisorySyncService.RegisterScopeDemand(ctx, "ubuntu-22.04-x86_64")
	require.NoError(t, err)

	// Test GET /api/v1/advisories/overview
	overviewRecorder := backend.HTTPGet("/api/v1/advisories/overview", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, overviewRecorder.Code)

	var overview map[string]any
	require.NoError(t, json.Unmarshal(overviewRecorder.Body.Bytes(), &overview))
	assert.Contains(t, overview, "total_advisories")
	assert.Contains(t, overview, "total_scopes")
	assert.Contains(t, overview, "synced_scopes")
	assert.Equal(t, float64(1), overview["total_scopes"]) // We demanded 1 scope

	// Test GET /api/v1/advisories/scopes
	scopesRecorder := backend.HTTPGet("/api/v1/advisories/scopes", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, scopesRecorder.Code)

	var scopes []map[string]any
	require.NoError(t, json.Unmarshal(scopesRecorder.Body.Bytes(), &scopes))
	require.Len(t, scopes, 1)
	assert.Equal(t, "ubuntu-22.04-x86_64", scopes[0]["scope_key"])
	assert.Equal(t, "pending", scopes[0]["status"])

	// Test POST /api/v1/advisories/scopes/{scopeKey}/sync
	syncRecorder := backend.HTTPPost(
		"/api/v1/advisories/scopes/ubuntu-22.04-x86_64/sync",
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, syncRecorder.Code)

	var syncResp map[string]any
	require.NoError(t, json.Unmarshal(syncRecorder.Body.Bytes(), &syncResp))
	assert.Equal(t, "pending", syncResp["status"])
}

func TestAdvisoryEndpoints_ManualSyncMissingScope(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Test POST /api/v1/advisories/scopes/{scopeKey}/sync for an unknown scope
	// This should register it on-demand and trigger the sync
	syncRecorder := backend.HTTPPost(
		"/api/v1/advisories/scopes/debian:bookworm-dsa/sync",
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, syncRecorder.Code)

	var syncResp map[string]any
	require.NoError(t, json.Unmarshal(syncRecorder.Body.Bytes(), &syncResp))
	assert.Equal(t, "pending", syncResp["status"])

	// Assert the scope was actually created in pending state
	scopesRecorder := backend.HTTPGet("/api/v1/advisories/scopes", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, scopesRecorder.Code)

	var scopes []map[string]any
	require.NoError(t, json.Unmarshal(scopesRecorder.Body.Bytes(), &scopes))
	require.Len(t, scopes, 1)
	assert.Equal(t, "debian:bookworm-dsa", scopes[0]["scope_key"])
	assert.Equal(t, "pending", scopes[0]["status"])
}

type failingAdvisorySyncService struct {
	services.AdvisorySyncService
}

func (f *failingAdvisorySyncService) TriggerManualSync(ctx context.Context, scopeKey string) error {
	return fmt.Errorf("mock manual sync failure")
}

func TestAdvisoryEndpoints_ErrorPathInternalError(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.AdvisorySyncService](i, func(i do.Injector) (services.AdvisorySyncService, error) {
				return &failingAdvisorySyncService{}, nil
			})
		}),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	syncRecorder := backend.HTTPPost(
		"/api/v1/advisories/scopes/debian:bookworm-dsa/sync",
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusInternalServerError, syncRecorder.Code)

	var errBody map[string]any
	require.NoError(t, json.Unmarshal(syncRecorder.Body.Bytes(), &errBody))
	assert.Equal(t, "failed to trigger manual sync", errBody["error"])
}

func TestAdvisoryEndpoints_ManualSync_Repeated(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	pool := do.MustInvoke[*pgxpool.Pool](backend.Injector())
	ctx := context.Background()

	// Clear any pre-existing advisory_sync jobs
	_, err = pool.Exec(ctx, "DELETE FROM river_job WHERE kind = 'advisory_sync'")
	require.NoError(t, err)

	scopeKey := "ubuntu-22.04-x86_64"

	// Trigger manual sync twice
	for i := 0; i < 2; i++ {
		syncRecorder := backend.HTTPPost(
			fmt.Sprintf("/api/v1/advisories/scopes/%s/sync", scopeKey),
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusOK, syncRecorder.Code)

		var syncResp map[string]any
		require.NoError(t, json.Unmarshal(syncRecorder.Body.Bytes(), &syncResp))
		assert.Equal(t, "pending", syncResp["status"])
	}

	// Verify that the count of advisory_sync jobs in the river_job table is exactly 1.
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'advisory_sync'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetAdvisory(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	pool := do.MustInvoke[*pgxpool.Pool](backend.Injector())
	ctx := context.Background()

	// Insert test advisory
	_, err = pool.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES ('USN-1234-1', 'ubuntu_usn_api', '1234-1', 'ubuntu', 'security', 'high', 'Test advisory', 'vendor_db', true, now())
	`)
	require.NoError(t, err)

	// Test GET success
	rec1 := backend.HTTPGet("/api/v1/advisories/USN-1234-1", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, rec1.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &payload))
	assert.Equal(t, "USN-1234-1", payload["id"])
	assert.Equal(t, "Test advisory", payload["summary"])
	assert.Equal(t, "high", payload["severity"])

	// Test GET missing
	rec2 := backend.HTTPGet("/api/v1/advisories/MISSING-9999", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusNotFound, rec2.Code)

	// Test GET unauthorized
	rec3 := backend.HTTPGet("/api/v1/advisories/USN-1234-1")
	require.Equal(t, http.StatusUnauthorized, rec3.Code)
}
