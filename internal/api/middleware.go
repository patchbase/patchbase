package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/utils"
)

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.ResponseWriter.Write(body)
}

func (w *statusResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}

	return w.status
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers.Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdnjs.cloudflare.com; font-src 'self' https://fonts.gstatic.com https://cdnjs.cloudflare.com data:; img-src 'self' data: blob:; connect-src 'self'; frame-ancestors 'none'",
		)
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		headers.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
			headers.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

type requestIDContextKey struct{}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func GetRequestID(ctx context.Context) string {
	requestID, ok := ctx.Value(requestIDContextKey{}).(string)
	if !ok {
		return ""
	}

	return requestID
}

func RequestContextMiddleware(logger *slog.Logger, rg utils.RandomStringGenerator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := rg.Hex(8)

		requestLogger := logger.With("request_id", requestID)
		w.Header().Set(webutil.RequestIDHeader, requestID)

		ctx := withRequestID(r.Context(), requestID)
		ctx = utils.WithLogger(ctx, requestLogger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func LoggingMiddleware(level string, next http.Handler) http.Handler {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseWriter := &statusResponseWriter{
			ResponseWriter: w,
			status:         0,
		}
		start := time.Now()
		next.ServeHTTP(responseWriter, r)
		if shouldSkipAccessLog(r.URL.Path) {
			return
		}
		logger := utils.GetLogger(r.Context())
		logger.Log(
			r.Context(),
			slogLevel,
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", responseWriter.Status(),
			"duration", time.Since(start),
		)
	})
}

func shouldSkipAccessLog(path string) bool {
	return strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/")
}
