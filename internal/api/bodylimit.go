// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package api

import "net/http"

// DefaultMaxRequestBodyBytes is the fallback cap applied when a route
// does not specify one. Sized for typical host snapshot uploads with
// headroom; raise via the api.max_request_body_bytes config knob.
const DefaultMaxRequestBodyBytes int64 = 32 * 1024 * 1024

// MaxBodyBytesMiddleware caps inbound request body size by wrapping
// r.Body with http.MaxBytesReader. When a downstream reader trips the
// cap it returns *http.MaxBytesError; handlers should detect this and
// respond with HTTP 413 (e.g. via webutil.WriteAPIError).
//
// A non-positive limit falls back to DefaultMaxRequestBodyBytes so the
// cap can never be silently disabled by misconfiguration.
func MaxBodyBytesMiddleware(maxBytes int64) func(http.HandlerFunc) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRequestBodyBytes
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next(w, r)
		}
	}
}
