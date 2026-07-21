// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
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

func TestHosts_DeleteNegativePaths(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	t.Run("delete unknown host returns 404", func(t *testing.T) {
		recorder := backend.HTTPDelete(
			"/api/v1/hosts/h_nonexistent",
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("delete non-admin returns 403", func(t *testing.T) {
		recorder := backend.HTTPDelete(
			"/api/v1/hosts/h_nonexistent",
			apitesting.WithBearerToken(userToken),
		)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("delete unauthenticated returns 401", func(t *testing.T) {
		recorder := backend.HTTPDelete("/api/v1/hosts/h_nonexistent")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("delete host with associated snapshots jobs and tokens", func(t *testing.T) {
		ctx := context.Background()
		queries := db.New(backend.DB())

		hostID := id.New("h")
		_, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
			ID:           hostID,
			DisplayName:  utils.Some("delete-with-data"),
			MachineID:    utils.Some("delete-with-data-machine"),
			Hostname:     utils.Some("delete-with-data"),
			IpAddress:    utils.Some("10.0.0.97"),
			OsName:       "Ubuntu",
			OsVersion:    "22.04",
			Architecture: "x86_64",
		})
		require.NoError(t, err)

		_, err = queries.ApproveHostByID(ctx, hostID)
		require.NoError(t, err)

		snapshotID := id.New("snap")
		_, err = backend.DB().Exec(ctx, `
			INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
			VALUES ($1, $2, now(), $3, '5.15.0-100-generic', NULL, false)
		`, snapshotID, hostID, []byte("{}"))
		require.NoError(t, err)

		_, err = backend.DB().Exec(ctx, `
			INSERT INTO host_current_state (host_id, snapshot_id, overall_action, critical_count, updated_at)
			VALUES ($1, $2, 'update_package', 1, now())
		`, hostID, snapshotID)
		require.NoError(t, err)

		tokenID := id.New("htok")
		_, err = queries.InsertHostAccessToken(ctx, db.InsertHostAccessTokenParams{
			ID:        tokenID,
			HostID:    hostID,
			TokenHash: utils.SHA256("test-token"),
		})
		require.NoError(t, err)

		_, err = backend.DB().Exec(ctx, `
			INSERT INTO host_ssh_pull_jobs (id, host_id, status, started_at)
			VALUES ($1, $2, 'success', now())
		`, id.New("j"), hostID)
		require.NoError(t, err)

		deleteRecorder := backend.HTTPDelete(
			fmt.Sprintf("/api/v1/hosts/%s", hostID),
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusOK, deleteRecorder.Code)

		var deletePayload map[string]any
		require.NoError(t, json.Unmarshal(deleteRecorder.Body.Bytes(), &deletePayload))
		assert.Equal(t, true, deletePayload["ok"])

		hostRecorder := backend.HTTPGet(
			fmt.Sprintf("/api/v1/hosts/%s", hostID),
			apitesting.WithBearerToken(adminToken),
		)
		require.Equal(t, http.StatusNotFound, hostRecorder.Code)
	})
}
