package agent

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	agent "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func Register(i do.Injector) http.HandlerFunc {
	hosts := do.MustInvoke[services.Hosts](i)

	return webutil.ValidateNoAuth(func(w http.ResponseWriter, r *http.Request, req *agent.RegisterHostRequest) {
		result, err := hosts.RegisterAgentHost(r.Context(), req)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrInvalidRegistrationToken):
				webutil.WriteAPIError(w, r, http.StatusUnauthorized, "invalid registration token", nil)
			case errors.Is(err, services.ErrDuplicateHostDisplayName):
				webutil.WriteAPIError(w, r, http.StatusConflict, "host with this name already exists", nil)
			default:
				webutil.LogError(r, "register agent host failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "registration failed", nil)
			}
			return
		}

		webutil.WriteProto(w, http.StatusCreated, result)
	})
}
