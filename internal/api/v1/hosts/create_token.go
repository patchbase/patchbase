package hosts

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type createTokenRequest struct {
	Name string `json:"name"`
}

func CreateToken(i do.Injector) apiauth.AuthenticatedHandler {
	hosts := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can create registration tokens", nil)
			return
		}

		var req createTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", nil)
			return
		}

		created, err := hosts.CreateRegistrationToken(r.Context(), authInfo.User.ID, req.Name)
		if err != nil {
			webutil.LogError(r, "create registration token failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to create registration token", nil)
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, entities.NewCreatedRegistrationToken(created))
	}
}
