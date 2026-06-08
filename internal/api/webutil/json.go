package webutil

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	agent "go.patchbase.net/proto/agent"
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
	json.NewEncoder(w).Encode(payload) // nolint:errcheck
}

func WriteAPIError(w http.ResponseWriter, r *http.Request, status int, message string, details any) {
	if status >= http.StatusInternalServerError {
		reqLogger := utils.GetLogger(r.Context())
		reqLogger.ErrorContext(r.Context(), "internal server error", "public_message", message, "details", details)
		message = "internal server error"
		details = nil
	}
	if r.Header.Get("Content-Type") == "application/x-protobuf" || r.Header.Get("Accept") == "application/x-protobuf" {
		if details != nil {
			message = fmt.Sprintf("%s: %v", message, details)
		}
		WriteProto(w, status, &agent.APIError{Error: message})
		return
	}
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
