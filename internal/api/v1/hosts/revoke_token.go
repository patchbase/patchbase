// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

func RevokeToken(i do.Injector) apiauth.AuthenticatedHandler {
	hosts := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenRevokeToken)
			return
		}

		tokenID := r.PathValue("tokenID")
		if tokenID == "" {
			webutil.WriteError(w, r, apperr.ErrMissingTokenID)
			return
		}

		if err := hosts.RevokeRegistrationToken(r.Context(), tokenID); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
