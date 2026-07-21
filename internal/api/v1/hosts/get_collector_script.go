// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/services"
)

func GetCollectorScript(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		osFamily := strings.TrimSpace(r.URL.Query().Get("os_family"))
		if osFamily == "" {
			http.Error(w, "missing required query parameter: os_family", http.StatusBadRequest)
			return
		}

		script, err := hostsService.GetCollectorScript(osFamily)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		scriptBytes := []byte(script)

		w.Header().Set("Content-Type", "text/x-shellscript")
		w.Header().Set("Content-Disposition", `attachment; filename="patchbase-collector.sh"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(scriptBytes)))
		w.WriteHeader(http.StatusOK)
		w.Write(scriptBytes) // nolint: errcheck, gosec
	}
}
