package settings

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type getSettingsResponse struct {
	GlobalSSHPublicKey string `json:"global_ssh_public_key"`
}

func GetSettings(i do.Injector) apiauth.AuthenticatedHandler {
	settingsService := do.MustInvoke[services.Settings](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can access settings", nil)
			return
		}

		globalKey, err := settingsService.GetGlobalSSHKeyPair(r.Context())
		if err != nil {
			webutil.LogError(r, "get global ssh key failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to retrieve global SSH key", nil)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, getSettingsResponse{
			GlobalSSHPublicKey: globalKey.PublicKey,
		})
	}
}
