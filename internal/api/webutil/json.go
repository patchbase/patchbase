package webutil

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"go.patchbase.net/server/internal/utils"
)

const RequestIDHeader = "X-Request-ID"

type APIError struct {
	Error   string `json:"error"`
	Details any    `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteAPIError(w http.ResponseWriter, r *http.Request, status int, message string, details any) {
	WriteJSON(w, status, APIError{
		Error:   message,
		Details: details,
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
