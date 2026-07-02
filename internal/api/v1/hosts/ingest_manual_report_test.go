package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	"go.patchbase.net/server/internal/config"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
)

func TestIngestManualReport(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	ctx := context.Background()
	queries := db.New(backend.DB())

	hostID := id.New("h")
	_, err = queries.InsertManualHost(ctx, db.InsertManualHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("manual-host"),
		Hostname:     utils.Some("manual-host"),
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	reportContent := `_PB_METADATA_HOSTNAME=manual-host
_PB_METADATA_ARCH=x86_64
_PB_METADATA_KERNEL=5.15.0
_PB_METADATA_MACHINE_ID=manual-machine
_PB_METADATA_IP=10.0.0.50
_PB_METADATA_BOOT_TIME=1672531199
_PB_METADATA_OS_ID=ubuntu
_PB_METADATA_OS_ID_LIKE=debian
_PB_METADATA_OS_NAME=Ubuntu
_PB_METADATA_OS_VERSION=22.04
---UPDATES_START---
bash amd64 5.1-6ubuntu2
---PACKAGES_START---
bash|5.1|6ubuntu1|amd64||
---REPOS_START---
archive.ubuntu.com Enabled`
	payloadBytes := []byte(reportContent)

	t.Run("anonymous returns 401", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
			payloadBytes,
		)
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("non-admin returns 403", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
			payloadBytes,
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("admin succeeds", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
			payloadBytes,
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusOK, recorder.Code)
		
		var resp map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
		assert.Equal(t, "success", resp["status"])
	})

	t.Run("invalid host returns 400 or error", func(t *testing.T) {
		recorder := backend.HTTPPostBytes(
			"/api/v1/hosts/non-existent/report",
			payloadBytes,
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestIngestManualReport_OversizedBodyReturns413(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			cfg := do.MustInvoke[config.Config](i)
			cfg.API.MaxRequestBodyBytes = 256
			do.Override[config.Config](i, func(_ do.Injector) (config.Config, error) {
				return cfg, nil
			})
		}),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	ctx := context.Background()
	queries := db.New(backend.DB())

	hostID := id.New("h")
	_, err = queries.InsertManualHost(ctx, db.InsertManualHostParams{
		ID:          hostID,
		DisplayName: utils.Some("manual-host-cap"),
		Hostname:    utils.Some("manual-host-cap"),
	})
	require.NoError(t, err)
	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	oversized := strings.Repeat("x", 1024)
	recorder := backend.HTTPPostBytes(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		[]byte(oversized),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
}
