package hosts

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type createSSHHostRequest struct {
	DisplayName      string `json:"display_name"`
	Hostname         string `json:"hostname"`
	SSHUser          string `json:"ssh_user"`
	FrequencyMinutes int32  `json:"frequency_minutes"`
}

type createSSHHostResponse struct {
	HostID         string `json:"host_id"`
	PublicKey      string `json:"public_key"`
	ApprovalStatus string `json:"approval_status"`
	LastRunStatus  string `json:"last_run_status"`
	LastRunError   string `json:"last_run_error"`
}

func CreateSSHHost(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can create ssh hosts", nil)
			return
		}

		var req createSSHHostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", nil)
			return
		}

		result, err := hostsService.CreateSSHHost(r.Context(), services.CreateSSHHostInput{
			DisplayName:      req.DisplayName,
			Hostname:         req.Hostname,
			SSHUser:          req.SSHUser,
			FrequencyMinutes: req.FrequencyMinutes,
		})
		if err != nil {
			webutil.LogError(r, "create ssh host failed", err)
			webutil.WriteAPIError(w, r, http.StatusBadRequest, err.Error(), nil)
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, createSSHHostResponse{
			HostID:         result.HostID,
			PublicKey:      result.PublicKey,
			ApprovalStatus: result.ApprovalStatus,
			LastRunStatus:  result.LastRunStatus,
			LastRunError:   result.LastRunError,
		})
	}
}
