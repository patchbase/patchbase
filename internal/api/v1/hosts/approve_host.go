package hosts

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func Approve(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can approve hosts", nil)
			return
		}

		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host id", nil)
			return
		}

		host, err := hostsService.ApproveHost(r.Context(), hostID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrHostNotFound):
				webutil.WriteAPIError(w, r, http.StatusNotFound, "host not found", nil)
			default:
				webutil.LogError(r, "approve host failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to approve host", nil)
			}
			return
		}

		webutil.WriteJSON(w, http.StatusOK, entities.NewHost(host))
	}
}
