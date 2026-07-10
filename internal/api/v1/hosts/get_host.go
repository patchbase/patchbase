package hosts

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func GetHost(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteError(w, r, apperr.ErrMissingHostID)
			return
		}

		host, err := hostsService.GetHost(r.Context(), hostID)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, entities.NewHost(host))
	}
}