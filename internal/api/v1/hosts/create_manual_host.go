package hosts

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
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
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can create manual hosts", nil)
			return
		}

		var req createManualHostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", nil)
			return
		}

		result, err := hostsService.CreateManualHost(r.Context(), req.DisplayName, req.Hostname)
		if err != nil {
			webutil.LogError(r, "create manual host failed", err)
			webutil.WriteAPIError(w, r, http.StatusBadRequest, err.Error(), nil)
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, createManualHostResponse{
			HostID:         result.ID,
			ApprovalStatus: result.ApprovalStatus,
		})
	}
}
