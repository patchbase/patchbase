package dashboard

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func GetOverview(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		overview, err := hostsService.GetDashboardOverview(r.Context())
		if err != nil {
			webutil.LogError(r, "get dashboard overview failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get dashboard overview", nil)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, overview)
	}
}
