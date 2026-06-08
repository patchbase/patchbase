package hosts_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
)

func TestHosts_OnboardSSHNegativePaths(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	t.Run("onboard unknown host returns 404", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/h_nonexistent/onboard-ssh",
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("onboard non-admin returns 403", func(t *testing.T) {
		recorder := backend.HTTPPost(
			"/api/v1/hosts/h_nonexistent/onboard-ssh",
			"{}",
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("onboard unauthenticated returns 401", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/hosts/h_nonexistent/onboard-ssh", "{}")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("onboard non-SSH host returns 500", func(t *testing.T) {
		ctx := context.Background()
		queries := db.New(backend.DB())
		hostID := id.New("h")
		_, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
			ID:           hostID,
			DisplayName:  utils.Some("not-ssh-host"),
			MachineID:    utils.Some("not-ssh-machine"),
			Hostname:     utils.Some("not-ssh"),
			IpAddress:    utils.Some("10.0.0.98"),
			OsName:       "Ubuntu",
			OsVersion:    "22.04",
			Architecture: "x86_64",
		})
		require.NoError(t, err)

		recorder := backend.HTTPPost(
			fmt.Sprintf("/api/v1/hosts/%s/onboard-ssh", hostID),
			"{}",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}
