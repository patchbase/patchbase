package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
)

func TestHosts_ApproveNegativePaths(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	t.Run("approve unknown host returns 404", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/h_nonexistent/approve",
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("approve non-admin returns 403", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/h_nonexistent/approve",
			"{}",
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("approve unauthenticated returns 401", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/hosts/h_nonexistent/approve", "{}")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("approve already-approved host is idempotent", func(t *testing.T) {
		ctx := context.Background()
		queries := db.New(backend.DB())
		hostID := id.New("h")
		_, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
			ID:           hostID,
			DisplayName:  utils.Some("approve-twice-host"),
			MachineID:    utils.Some("approve-twice-machine"),
			Hostname:     utils.Some("approve-twice"),
			IpAddress:    utils.Some("10.0.0.99"),
			OsName:       "Ubuntu",
			OsVersion:    "22.04",
			Architecture: "x86_64",
		})
		require.NoError(t, err)

		first := backend.HTTPPost(
			fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusOK, first.Code)
		var firstPayload map[string]any
		require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstPayload))
		assert.Equal(t, "approved", firstPayload["approval_status"])

		second := backend.HTTPPost(
			fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusOK, second.Code)
		var secondPayload map[string]any
		require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondPayload))
		assert.Equal(t, "approved", secondPayload["approval_status"])
	})
}
