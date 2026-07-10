package hosts

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func RunPullNow(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenRunSSHPull)
			return
		}

		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteError(w, r, apperr.ErrMissingHostID)
			return
		}

		if err := hostsService.RunSSHPull(r.Context(), hostID); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}