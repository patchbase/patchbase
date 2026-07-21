// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package webutil

import (
	"log/slog"
	"net/http"

	"google.golang.org/protobuf/proto"
)

func WriteProto(w http.ResponseWriter, status int, payload proto.Message) {
	respBytes, err := proto.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal proto payload", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(status)
	w.Write(respBytes) // nolint:errcheck
}
