// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

type updateHostRequest struct {
	DisplayName          utils.Option[string] `json:"display_name"`
	PullHostname         utils.Option[string] `json:"pull_hostname"`
	PullSSHUser          utils.Option[string] `json:"pull_ssh_user"`
	PullFrequencyMinutes utils.Option[int32]  `json:"pull_frequency_minutes"`
}

func UpdateHost(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)
	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenUpdateHost)
			return
		}

		var req updateHostRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidBody, err))
			return
		}
		req.DisplayName = req.DisplayName.Map(strings.TrimSpace)
		if displayName, ok := req.DisplayName.Get(); ok && displayName == "" {
			webutil.WriteError(w, r, apperr.ErrDisplayNameRequired)
			return
		}
		req.PullHostname = req.PullHostname.Map(strings.TrimSpace)
		if pullHostname, ok := req.PullHostname.Get(); ok && pullHostname == "" {
			webutil.WriteError(w, r, apperr.ErrHostnameRequired)
			return
		}
		req.PullSSHUser = req.PullSSHUser.Map(strings.TrimSpace)
		if pullSSHUser, ok := req.PullSSHUser.Get(); ok && pullSSHUser == "" {
			webutil.WriteError(w, r, apperr.ErrSSHUserRequired)
			return
		}
		if pullFrequencyMinutes, ok := req.PullFrequencyMinutes.Get(); ok && pullFrequencyMinutes < 5 {
			webutil.WriteError(w, r, apperr.ErrInvalidFrequency)
			return
		}
		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteError(w, r, apperr.ErrHostNotFound)
			return
		}

		host, err := hostsService.UpdateHost(r.Context(), authInfo.ActorFromRequest(r), hostID, services.UpdateHostInput{
			DisplayName:          req.DisplayName,
			PullHostname:         req.PullHostname,
			PullSSHUser:          req.PullSSHUser,
			PullFrequencyMinutes: req.PullFrequencyMinutes,
		})
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}
		webutil.WriteJSON(w, http.StatusOK, entities.NewHost(host))
	}
}
