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

func ListPullJobs(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host id", nil)
			return
		}

		jobs, err := hostsService.ListSSHPullJobs(r.Context(), hostID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrHostNotFound):
				webutil.WriteAPIError(w, r, http.StatusNotFound, "host not found", nil)
			default:
				webutil.LogError(r, "list ssh pull jobs failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to list ssh pull jobs", nil)
			}
			return
		}

		webutil.WriteJSON(w, http.StatusOK, entities.NewHostSSHPullJobs(jobs))
	}
}
