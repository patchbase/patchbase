package hosts

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func DeleteHost(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can delete hosts", nil)
			return
		}

		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host id", nil)
			return
		}

		if err := hostsService.DeleteHost(r.Context(), hostID); err != nil {
			switch {
			case errors.Is(err, services.ErrHostNotFound):
				webutil.WriteAPIError(w, r, http.StatusNotFound, "host not found", nil)
			default:
				webutil.LogError(r, "delete host failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to delete host", nil)
			}
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
