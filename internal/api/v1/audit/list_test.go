// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package audit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
)

func TestAuditLogList_RequiresAuth(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)

	recorder := backend.HTTPGet("/api/v1/audit-logs")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuditLogList_ForbiddenForNonAdmin(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	recorder := backend.HTTPGet("/api/v1/audit-logs", apitesting.WithBearerToken(userToken))
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestAuditLogList_Empty(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	recorder := backend.HTTPGet("/api/v1/audit-logs", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 0, payload["total"])
	items, ok := payload["items"].([]any)
	require.True(t, ok)
	assert.Empty(t, items)
}

func TestAuditLogList_ReturnsRecordedEntries(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	queries := db.New(backend.DB())
	entries := []db.InsertAuditLogParams{
		{
			ID:         id.New("audit"),
			ActorID:    utils.Some("u_admin"),
			ActorEmail: "admin@patchbase.local",
			Action:     "host.create",
			TargetType: "host",
			TargetID:   utils.Some("h_demo"),
			Metadata:   []byte(`{"onboarding_mode":"manual"}`),
			IpAddress:  utils.Some("10.0.0.1"),
			UserAgent:  utils.Some("test-agent"),
		},
		{
			ID:         id.New("audit"),
			ActorID:    utils.None[string](),
			ActorEmail: "guest@example.com",
			Action:     "auth.login.failure",
			TargetType: "user",
			TargetID:   utils.None[string](),
			Metadata:   []byte(`{"reason":"invalid_credentials"}`),
			IpAddress:  utils.Some("10.0.0.2"),
			UserAgent:  utils.None[string](),
		},
	}
	for _, entry := range entries {
		require.NoError(t, queries.InsertAuditLog(ctx, entry))
	}

	recorder := backend.HTTPGet("/api/v1/audit-logs", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 2, payload["total"])
	items, ok := payload["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)

	byAction := make(map[string]map[string]any, 2)
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		require.True(t, ok)
		action, _ := item["action"].(string)
		byAction[action] = item
	}

	hostCreate, ok := byAction["host.create"]
	require.True(t, ok, "expected a host.create audit entry")
	assert.Equal(t, "host", hostCreate["target_type"])
	assert.Equal(t, "h_demo", hostCreate["target_id"])
	assert.Equal(t, "u_admin", hostCreate["actor_id"])
	assert.Equal(t, "admin@patchbase.local", hostCreate["actor_email"])

	metadata, ok := hostCreate["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be a JSON object, not a string")
	assert.Equal(t, "manual", metadata["onboarding_mode"])

	loginFailure, ok := byAction["auth.login.failure"]
	require.True(t, ok, "expected an auth.login.failure audit entry")
	assert.Equal(t, "user", loginFailure["target_type"])
	assert.Equal(t, "guest@example.com", loginFailure["actor_email"])
	assert.Nil(t, loginFailure["actor_id"], "expected anonymous actor_id for failed login")
}

func TestAuditLogList_Pagination(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	queries := db.New(backend.DB())
	for i := range 5 {
		require.NoError(t, queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
			ID:         id.New("audit"),
			ActorID:    utils.Some("u_admin"),
			ActorEmail: "admin@patchbase.local",
			Action:     fmt.Sprintf("host.pull.%d", i),
			TargetType: "host",
			TargetID:   utils.Some("h_demo"),
		}))
	}

	recorder := backend.HTTPGet("/api/v1/audit-logs?limit=2&offset=0", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 5, payload["total"])
	items, ok := payload["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 2)

	second := backend.HTTPGet("/api/v1/audit-logs?limit=2&offset=2", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, second.Code)
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &payload))
	items, ok = payload["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 2)
}

func TestAuditLogList_RejectsInvalidPagination(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	bad := backend.HTTPGet("/api/v1/audit-logs?limit=-1", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusBadRequest, bad.Code)

	bad = backend.HTTPGet("/api/v1/audit-logs?offset=foo", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusBadRequest, bad.Code)

	bad = backend.HTTPGet("/api/v1/audit-logs?from=not-a-date", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusBadRequest, bad.Code)

	bad = backend.HTTPGet("/api/v1/audit-logs?to=2025-13-99", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestAuditLogList_FilterByAction(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	queries := db.New(backend.DB())
	for i := range 4 {
		require.NoError(t, queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
			ID:         id.New("audit"),
			ActorID:    utils.Some("u_admin"),
			ActorEmail: "admin@patchbase.local",
			Action:     fmt.Sprintf("host.create.%d", i),
			TargetType: "host",
			TargetID:   utils.Some(fmt.Sprintf("h_%d", i)),
		}))
	}

	recorder := backend.HTTPGet("/api/v1/audit-logs?action=host.create.1", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 1, payload["total"])
	items, ok := payload["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "host.create.1", items[0].(map[string]any)["action"])

	recorder = backend.HTTPGet("/api/v1/audit-logs?action=does-not-exist", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 0, payload["total"])
	items, ok = payload["items"].([]any)
	require.True(t, ok)
	assert.Empty(t, items)
}

func TestAuditLogList_FilterByActor(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	queries := db.New(backend.DB())
	require.NoError(t, queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID:         id.New("audit"),
		ActorID:    utils.Some("u_admin"),
		ActorEmail: "admin@patchbase.local",
		Action:     "host.create",
		TargetType: "host",
		TargetID:   utils.Some("h_admin"),
	}))
	require.NoError(t, queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID:         id.New("audit"),
		ActorID:    utils.Some("u_user"),
		ActorEmail: "user@patchbase.local",
		Action:     "host.create",
		TargetType: "host",
		TargetID:   utils.Some("h_user"),
	}))

	recorder := backend.HTTPGet("/api/v1/audit-logs?actor=u_admin", apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 1, payload["total"])
	items, ok := payload["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.EqualValues(t, "u_admin", items[0].(map[string]any)["actor_id"])
}

func TestAuditLogList_FilterByDateRange(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	queries := db.New(backend.DB())
	require.NoError(t, queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID:         id.New("audit"),
		ActorID:    utils.Some("u_admin"),
		ActorEmail: "admin@patchbase.local",
		Action:     "host.create",
		TargetType: "host",
		TargetID:   utils.Some("h_one"),
	}))
	require.NoError(t, queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID:         id.New("audit"),
		ActorID:    utils.Some("u_admin"),
		ActorEmail: "admin@patchbase.local",
		Action:     "host.create",
		TargetType: "host",
		TargetID:   utils.Some("h_two"),
	}))

	farFuture := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	recorder := backend.HTTPGet("/api/v1/audit-logs?to="+farFuture, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 2, payload["total"])

	farFuture = time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	farPast := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	recorder = backend.HTTPGet(
		"/api/v1/audit-logs?from="+farPast+"&to="+farFuture,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 2, payload["total"])

	// Window that excludes the seeded entries: nothing should match.
	recorder = backend.HTTPGet(
		"/api/v1/audit-logs?to="+farPast,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 0, payload["total"])
}

// TestAuditLogList_DateOnlyToIncludesFullDay is a regression test for the bug where a bare
// YYYY-MM-DD value in `to` was parsed as midnight of that day, causing `<= to` to exclude every
// event recorded later in the selected day.
func TestAuditLogList_DateOnlyToIncludesFullDay(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	// Pin the entry to 23:30:00 UTC today so the test exercises "later in the same day"
	// the time that the bug used to drop on the floor.
	lateInDay := time.Now().UTC().Truncate(24 * time.Hour).
		Add(23*time.Hour + 30*time.Minute)

	_, err = backend.DB().Exec(ctx,
		`INSERT INTO audit_log (id, actor_id, actor_email, action, target_type, target_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id.New("audit"),
		"u_admin",
		"admin@patchbase.local",
		"host.create",
		"host",
		"h_late",
		lateInDay,
	)
	require.NoError(t, err)
	// And a baseline entry earlier in the day.
	_, err = backend.DB().Exec(ctx,
		`INSERT INTO audit_log (id, actor_id, actor_email, action, target_type, target_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id.New("audit"),
		"u_admin",
		"admin@patchbase.local",
		"host.create",
		"host",
		"h_early",
		lateInDay.Add(-22*time.Hour), // ~01:30 same day
	)
	require.NoError(t, err)

	dayStr := lateInDay.Format("2006-01-02")
	recorder := backend.HTTPGet("/api/v1/audit-logs?to="+dayStr, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 2, payload["total"], "date-only `to` must include entries created at 23:30 of the selected day")
}

// TestAuditLogList_RFC3339PreservesTimeOfDay is a regression test for the bug where
// RFC3339 timestamps in `from` / `to` were being silently widened to the start / end of their day.
// The result was that `to=YYYY-MM-DDThh:mm:ssZ` would include events recorded later in the day
// than the caller specified, and `from=...` would include events from earlier in the same day.
func TestAuditLogList_RFC3339PreservesTimeOfDay(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	ctx := context.Background()

	// Anchor three rows around a 14:30 UTC cutoff: one well before, one exactly at the cutoff, one well after.
	// The cutoff row is the boundary — RFC3339 `to` should include exactly two rows (the ones <= the cutoff instant).
	before := time.Date(2026, 7, 22, 14, 0, 0, 0, time.UTC)
	at := time.Date(2026, 7, 22, 14, 30, 0, 0, time.UTC)
	after := time.Date(2026, 7, 22, 15, 0, 0, 0, time.UTC)

	seed := func(suffix string, ts time.Time) {
		_, err := backend.DB().Exec(ctx,
			`INSERT INTO audit_log (id, actor_id, actor_email, action, target_type, target_id, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id.New("audit"),
			"u_admin",
			"admin@patchbase.local",
			"host.create",
			"host",
			"h_"+suffix,
			ts,
		)
		require.NoError(t, err)
	}
	seed("before", before)
	seed("at", at)
	seed("after", after)

	cutoff := at.Format(time.RFC3339)
	recorder := backend.HTTPGet("/api/v1/audit-logs?to="+cutoff, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 2, payload["total"], "RFC3339 `to` must NOT widen to end-of-day; only the cutoff and earlier rows should match")

	fromCutoff := before.Add(30 * time.Minute).Format(time.RFC3339)
	recorder = backend.HTTPGet("/api/v1/audit-logs?from="+fromCutoff, apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.EqualValues(t, 2, payload["total"], "RFC3339 `from` must NOT widen to start-of-day; only the cutoff and later rows should match")
}
