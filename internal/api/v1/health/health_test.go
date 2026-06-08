package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/api/v1/health"
	"go.patchbase.net/server/internal/buildinfo"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	health.Health(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))

	assert.Equal(t, "ok", payload["status"])
	assert.Equal(t, "patchbase", payload["service"])
	assert.Equal(t, buildinfo.Version, payload["version"])

	timestampStr, ok := payload["timestamp"].(string)
	require.True(t, ok)

	_, err := time.Parse(time.RFC3339, timestampStr)
	require.NoError(t, err)
}
