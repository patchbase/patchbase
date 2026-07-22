// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"errors"
	"io"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

type ingestManualReportResponse struct {
	Status string `json:"status"`
}

func IngestManualReport(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenIngestManualReport)
			return
		}

		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteError(w, r, apperr.ErrMissingHostID)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				webutil.WriteError(w, r, apperr.ErrBodyTooLarge)
				return
			}
			webutil.WriteError(w, r, apperr.ErrBodyReadFailed)
			return
		}

		if err := hostsService.IngestManualReport(r.Context(), authInfo.ActorFromRequest(r), hostID, body); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, ingestManualReportResponse{
			Status: "success",
		})
	}
}
