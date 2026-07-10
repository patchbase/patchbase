package advisories

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

func GetAdvisory(i do.Injector) apiauth.AuthenticatedHandler {
	svc := do.MustInvoke[services.AdvisorySyncService](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		id := r.PathValue("id")
		if id == "" {
			webutil.WriteError(w, r, apperr.ErrMissingAdvisoryID)
			return
		}

		adv, err := svc.GetAdvisory(r.Context(), id)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		resp := entities.MapAdvisory(adv)

		webutil.WriteJSON(w, http.StatusOK, resp)
	}
}
