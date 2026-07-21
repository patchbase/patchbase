// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package advisories

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func GetOverview(i do.Injector) apiauth.AuthenticatedHandler {
	advisoriesService := do.MustInvoke[services.AdvisorySyncService](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		overview, err := advisoriesService.GetOverview(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, overview)
	}
}
