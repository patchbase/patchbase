package hosts

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func ListHosts(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		hosts, err := hostsService.ListHosts(r.Context())
		if err != nil {
			webutil.LogError(r, "list hosts failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to list hosts", nil)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, entities.NewHosts(hosts))
	}
}
