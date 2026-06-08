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
	localAddrs := []string{"127.0.0.1:3000", "10.0.0.1:8080", "172.16.0.1:9090", "192.168.1.1:443"}
	nonLocalAddrs := []string{"8.8.8.8:1234", "203.0.113.1:5678", "198.51.100.1:80"}

	for _, addr := range localAddrs {
		t.Run("local="+addr, func(t *testing.T) {
			assertFullResponse(t, addr, "", "")
		})
	}

	for _, addr := range nonLocalAddrs {
		t.Run("nonlocal="+addr, func(t *testing.T) {
			assertMinimalResponse(t, addr, "", "")
		})
	}

	t.Run("X-Forwarded-For local", func(t *testing.T) {
		assertFullResponse(t, "10.0.0.1:443", "192.168.1.100", "")
	})

	t.Run("X-Forwarded-For nonlocal", func(t *testing.T) {
		assertMinimalResponse(t, "10.0.0.1:443", "8.8.8.8", "")
	})

	t.Run("X-Real-IP local", func(t *testing.T) {
		assertFullResponse(t, "10.0.0.1:443", "10.0.0.5", "")
	})

	t.Run("X-Real-IP nonlocal", func(t *testing.T) {
		assertMinimalResponse(t, "10.0.0.1:443", "", "198.51.100.1")
	})

	t.Run("X-Forwarded-For comma separated", func(t *testing.T) {
		assertFullResponse(t, "10.0.0.1:443", "127.0.0.1, 8.8.8.8", "")
	})

	t.Run("X-Forwarded-For spoofed", func(t *testing.T) {
		assertMinimalResponse(t, "8.8.8.8:1234", "127.0.0.1", "")
	})

	t.Run("missing remote addr", func(t *testing.T) {
		assertMinimalResponse(t, "", "", "")
	})
}

func assertFullResponse(t *testing.T, remoteAddr, xForwardedFor, xRealIP string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = remoteAddr
	if xForwardedFor != "" {
		req.Header.Set("X-Forwarded-For", xForwardedFor)
	}
	if xRealIP != "" {
		req.Header.Set("X-Real-IP", xRealIP)
	}
	rr := httptest.NewRecorder()
	health.Health(rr, req)
	assertFullResponsePayload(t, rr)
}

func assertMinimalResponse(t *testing.T, remoteAddr, xForwardedFor, xRealIP string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = remoteAddr
	if xForwardedFor != "" {
		req.Header.Set("X-Forwarded-For", xForwardedFor)
	}
	if xRealIP != "" {
		req.Header.Set("X-Real-IP", xRealIP)
	}
	rr := httptest.NewRecorder()
	health.Health(rr, req)
	assertMinimalResponsePayload(t, rr)
}

func assertFullResponsePayload(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
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

func assertMinimalResponsePayload(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusOK, rr.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))

	assert.Equal(t, "ok", payload["status"])
	assert.NotContains(t, payload, "service")
	assert.NotContains(t, payload, "version")
	assert.NotContains(t, payload, "timestamp")
}
