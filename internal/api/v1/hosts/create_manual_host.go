// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

type createManualHostRequest struct {
	DisplayName string `json:"display_name"`
	Hostname    string `json:"hostname"`
}

type createManualHostResponse struct {
	HostID         string `json:"host_id"`
	ApprovalStatus string `json:"approval_status"`
}

func CreateManualHost(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenCreateManualHost)
			return
		}

		var req createManualHostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		req.DisplayName = strings.TrimSpace(req.DisplayName)
		req.Hostname = strings.TrimSpace(req.Hostname)
		if req.DisplayName == "" && req.Hostname == "" {
			webutil.WriteError(w, r, apperr.ErrDisplayNameOrHostnameRequired)
			return
		}

		result, err := hostsService.CreateManualHost(r.Context(), req.DisplayName, req.Hostname)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, createManualHostResponse{
			HostID:         result.ID,
			ApprovalStatus: result.ApprovalStatus,
		})
	}
}
