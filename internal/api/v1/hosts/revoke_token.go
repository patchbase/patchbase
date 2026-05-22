package hosts

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func RevokeToken(i do.Injector) apiauth.AuthenticatedHandler {
	hosts := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can revoke registration tokens", nil)
			return
		}

		tokenID := r.PathValue("tokenID")
		if tokenID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing token id", nil)
			return
		}

		err := hosts.RevokeRegistrationToken(r.Context(), tokenID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrTokenAlreadyRevoked):
				webutil.WriteAPIError(w, r, http.StatusNotFound, "registration token not found or already revoked", nil)
			default:
				webutil.LogError(r, "revoke registration token failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to revoke registration token", nil)
			}
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
