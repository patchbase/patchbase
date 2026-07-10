package webutil

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	agent "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/utils"
)

const RequestIDHeader = "X-Request-ID"

type APIErrorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload) // nolint:errcheck
}

func wantsProto(r *http.Request) bool {
	return r.Header.Get("Content-Type") == "application/x-protobuf" ||
		r.Header.Get("Accept") == "application/x-protobuf"
}

// WriteError serializes err as a typed API error. It detects
// *apperr.Error via errors.As and emits the right shape for the
// requesting client (protobuf for agents, JSON for dashboard). Errors
// that do not implement *apperr.Error are logged and surfaced as a
// generic internal_error sentinel.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	ae, ok := errors.AsType[*apperr.Error](err)
	if !ok {
		LogError(r, "unhandled error", err)
		ae = apperr.ErrInternal
	}

	if ae.HTTPStatus >= 500 {
		LogError(r, "internal error", err)
		ae = &apperr.Error{
			HTTPStatus: ae.HTTPStatus,
			Code:       ae.Code,
			Message:    "internal server error",
			Details:    nil,
		}
	}

	if wantsProto(r) {
		WriteProto(w, ae.HTTPStatus, &agent.APIError{
			Code:    ae.Code,
			Message: ae.Message,
		})
		return
	}

	WriteJSON(w, ae.HTTPStatus, APIErrorJSON{
		Code:    ae.Code,
		Message: ae.Message,
		Details: ae.Details,
	})
}

func LogError(r *http.Request, message string, err error, attrs ...any) {
	logAttrs := append([]any{}, attrs...)
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
	}

	utils.GetLogger(r.Context()).ErrorContext(r.Context(), message, logAttrs...)
}

func RequestLogger(r *http.Request) *slog.Logger {
	return utils.GetLogger(r.Context())
}
