// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package health

import (
	"net/http"
	"time"

	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/buildinfo"
)

func Health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"status": "ok"}
	if webutil.IsLocalAddr(webutil.ClientIP(r)) {
		resp["service"] = "patchbase"
		resp["version"] = buildinfo.Version
		resp["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}
	webutil.WriteJSON(w, http.StatusOK, resp)
}
