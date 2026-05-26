package hosts

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func RunPullNow(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can run ssh pull jobs", nil)
			return
		}

		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host id", nil)
			return
		}

		err := hostsService.RunSSHPull(r.Context(), hostID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrHostNotFound):
				webutil.WriteAPIError(w, r, http.StatusNotFound, "host not found", nil)
			default:
				webutil.LogError(r, "run ssh pull now failed", err)
				webutil.WriteAPIError(w, r, http.StatusBadRequest, err.Error(), nil)
			}
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
