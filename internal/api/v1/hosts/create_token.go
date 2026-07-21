// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

type createTokenRequest struct {
	Name string `json:"name"`
}

func CreateToken(i do.Injector) apiauth.AuthenticatedHandler {
	hosts := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenCreateToken)
			return
		}

		var req createTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		created, err := hosts.CreateRegistrationToken(r.Context(), authInfo.User.ID, req.Name)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, entities.NewCreatedRegistrationToken(created))
	}
}
