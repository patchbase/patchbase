// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package lifecycle

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func GetCatalogSource(i do.Injector) apiauth.AuthenticatedHandler {
	lifecycleService := do.MustInvoke[services.LifecycleService](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		source, err := lifecycleService.CatalogSource(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, source)
	}
}
