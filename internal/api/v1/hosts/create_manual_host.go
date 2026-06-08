package hosts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

		req.DisplayName = strings.TrimSpace(req.DisplayName)
		req.Hostname = strings.TrimSpace(req.Hostname)
		if req.DisplayName == "" && req.Hostname == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "display name or hostname is required", nil)
			return
		}

		result, err := hostsService.CreateManualHost(r.Context(), req.DisplayName, req.Hostname)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrDuplicateHostDisplayName):
				webutil.WriteAPIError(w, r, http.StatusBadRequest, err.Error(), nil)
			default:
				webutil.LogError(r, "create manual host failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to create manual host", nil)
			}
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, createManualHostResponse{
			HostID:         result.ID,
			ApprovalStatus: result.ApprovalStatus,
		})
	}
}
