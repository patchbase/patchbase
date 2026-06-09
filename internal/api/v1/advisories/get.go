package advisories

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func GetAdvisory(i do.Injector) apiauth.AuthenticatedHandler {
	svc := do.MustInvoke[services.AdvisorySyncService](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		id := r.PathValue("id")
		if id == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing advisory id", nil)
			return
		}

		adv, err := svc.GetAdvisory(r.Context(), id)
		if err != nil {
			if errors.Is(err, services.ErrAdvisoryNotFound) {
				webutil.WriteAPIError(w, r, http.StatusNotFound, "advisory not found", nil)
				return
			}
			webutil.LogError(r, "failed to get advisory", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get advisory", nil)
			return
		}

		resp := entities.MapAdvisory(adv)

		webutil.WriteJSON(w, http.StatusOK, resp)
	}
}
