package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestListPullJobsReturnsLastTenEntries(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createRecorder := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"ssh-pull-history","hostname":"203.0.113.11","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &payload))
	hostID := payload["host_id"].(string)

	baseStartedAt := time.Now().UTC().Add(-12 * time.Minute)
	for i := 0; i < 12; i++ {
		_, err := backend.DB().Exec(context.Background(), `
			INSERT INTO host_ssh_pull_jobs (id, host_id, status, started_at)
			VALUES ($1, $2, 'success', $3)
		`, fmt.Sprintf("j_%02d", i), hostID, baseStartedAt.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	jobsRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/pull-jobs", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, jobsRecorder.Code)
	var jobs []map[string]any
	require.NoError(t, json.Unmarshal(jobsRecorder.Body.Bytes(), &jobs))
	require.Len(t, jobs, 10)
	assert.Equal(t, "j_11", jobs[0]["id"])
	assert.Equal(t, "j_02", jobs[9]["id"])
	for _, job := range jobs {
		assert.NotEqual(t, "j_00", job["id"])
		assert.NotEqual(t, "j_01", job["id"])
	}
}
