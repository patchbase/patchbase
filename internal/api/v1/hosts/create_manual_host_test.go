package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestHosts_UniqueConstraints_API(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	// Test unique SSH hosts
	// 1. Create first SSH host
	rec1 := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"api-ssh-host-1","hostname":"api-ssh-1.example.com","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec1.Code)

	// 2. Try to create second SSH host with same display name
	rec2 := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"api-ssh-host-1","hostname":"api-ssh-2.example.com","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "a host with this display name already exists")

	// 3. Try to create second SSH host with same pull hostname
	rec3 := backend.HTTPPost(
		"/api/v1/hosts/ssh",
		`{"display_name":"api-ssh-host-2","hostname":"api-ssh-1.example.com","ssh_user":"root","frequency_minutes":60}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "an SSH host with this pull hostname already exists")

	// Test unique manual hosts
	// 1. Create first manual host
	rec4 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-1","hostname":"api-manual-1.example.com"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec4.Code)

	// 2. Try to create second manual host with same display name
	rec5 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-1","hostname":"api-manual-2.example.com"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec5.Code)
	assert.Contains(t, rec5.Body.String(), "a host with this display name already exists")
}

func TestManualHosts_API(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)
	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	// Create manual host - success
	rec1 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-new","hostname":"api-manual-new.example.com"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, rec1.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &payload))
	hostID := payload["host_id"].(string)
	assert.NotEmpty(t, hostID)

	// Create manual host - non-admin forbidden
	rec2 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"api-manual-host-2","hostname":"api-manual-2.example.com"}`,
		apitesting.WithBearerToken(userToken),
	)
	require.Equal(t, http.StatusForbidden, rec2.Code)

	// Create manual host - bad JSON
	rec3 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec3.Code)

	// Create manual host - validation failure
	rec4 := backend.HTTPPost(
		"/api/v1/hosts/manual",
		`{"display_name":"","hostname":""}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec4.Code)

	// Ingest manual report - success
	reportContent := `_PB_METADATA_HOSTNAME=api-manual-new.example.com
_PB_METADATA_ARCH=x86_64
_PB_METADATA_OS_ID=rocky
_PB_METADATA_OS_NAME=Rocky Linux
_PB_METADATA_OS_VERSION=9.3
---UPDATES_START---
---PACKAGES_START---
---REPOS_START---`

	rec5 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		reportContent,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, rec5.Code)

	// Ingest manual report - non-admin forbidden
	rec6 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		reportContent,
		apitesting.WithBearerToken(userToken),
	)
	require.Equal(t, http.StatusForbidden, rec6.Code)

	// Ingest manual report - missing/unknown host
	rec7 := backend.HTTPPost(
		"/api/v1/hosts/h_unknown123/report",
		reportContent,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusBadRequest, rec7.Code)

	// Ingest manual report - invalid payload
	rec8 := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/report", hostID),
		`INVALID_REPORT_DATA`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, rec8.Code)

	// resulting snapshot/decision effects
	snapshotRecorder := backend.HTTPGet(fmt.Sprintf("/api/v1/hosts/%s/snapshot", hostID), apitesting.WithBearerToken(adminToken))
	require.Equal(t, http.StatusOK, snapshotRecorder.Code)
	var snapshotPayload map[string]any
	require.NoError(t, json.Unmarshal(snapshotRecorder.Body.Bytes(), &snapshotPayload))
	assert.Equal(t, hostID, snapshotPayload["host_id"])
}
