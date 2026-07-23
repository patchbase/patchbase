// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

type updateHostNotesRequest struct {
	Notes json.RawMessage `json:"notes"`
}

func UpdateHostNotes(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)
	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenUpdateHostNotes)
			return
		}

		var req updateHostNotesRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				webutil.WriteError(w, r, apperr.ErrBodyTooLarge)
				return
			}
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}
		if req.Notes == nil {
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		notes := utils.None[string]()
		if !bytes.Equal(req.Notes, []byte("null")) {
			var value string
			if err := json.Unmarshal(req.Notes, &value); err != nil {
				webutil.WriteError(w, r, apperr.ErrInvalidBody)
				return
			}
			notes = utils.Some(value)
		}

		host, err := hostsService.UpdateHostNotes(r.Context(), r.PathValue("hostID"), notes)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}
		webutil.WriteJSON(w, http.StatusOK, entities.NewHost(host))
	}
}
