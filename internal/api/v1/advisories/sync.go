package advisories

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func TriggerSync(i do.Injector) apiauth.AuthenticatedHandler {
	advisoriesService := do.MustInvoke[services.AdvisorySyncService](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		scopeKey := r.PathValue("scopeKey")
		if scopeKey == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing scope key", nil)
			return
		}

		err := advisoriesService.TriggerManualSync(r.Context(), scopeKey)
		if err != nil {
			webutil.LogError(r, "trigger manual sync failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to trigger manual sync", nil)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "pending"})
	}
}
