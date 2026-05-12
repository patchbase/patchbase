package api

import (
	"bytes"
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.patchbase.net/server/internal/api/webutil"
)

type fixedIDGenerator struct {
	id string
}

func (g fixedIDGenerator) New(int) string {
	return g.id
}

func (g fixedIDGenerator) Hex(int) string {
	return g.id
}

func TestRequestContextMiddlewareAddsRequestIDAndLoggingMiddlewareLogsStatus(t *testing.T) {
	buffer := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buffer, nil))

	handler := RequestContextMiddleware(
		logger,
		fixedIDGenerator{id: "req_test"},
		LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})),
	)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/snapshots", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, "req_test", recorder.Header().Get(webutil.RequestIDHeader))
	assert.Contains(t, buffer.String(), "request_id=req_test")
	assert.Contains(t, buffer.String(), "status=201")
}

func TestRequestContextMiddlewareAlwaysGeneratesNewRequestID(t *testing.T) {
	handler := RequestContextMiddleware(
		slog.Default(),
		fixedIDGenerator{id: "req_generated"},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/hosts", nil)
	request.Header.Set(webutil.RequestIDHeader, "req_incoming")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, "req_generated", recorder.Header().Get(webutil.RequestIDHeader))
}

func TestLoggingMiddlewareSkipsStaticRequests(t *testing.T) {
	buffer := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buffer, nil))

	handler := RequestContextMiddleware(
		logger,
		fixedIDGenerator{id: "req_static"},
		LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	request := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "req_static", recorder.Header().Get(webutil.RequestIDHeader))
	assert.NotContains(t, buffer.String(), "http request")
}

func TestSecurityHeadersMiddlewareSetsHeaders(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/hosts", nil)
	request.TLS = &tls.ConnectionState{}
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, "default-src 'self'; style-src 'self' https://cdnjs.cloudflare.com; font-src https://cdnjs.cloudflare.com; frame-ancestors 'none'", recorder.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"))
	assert.Equal(t, "geolocation=(), microphone=(), camera=()", recorder.Header().Get("Permissions-Policy"))
	assert.Equal(t, "max-age=63072000; includeSubDomains", recorder.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeadersMiddlewareSkipsHSTSOnPlainHTTP(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/hosts", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Empty(t, recorder.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
}
