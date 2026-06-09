package settings

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type getSettingsResponse struct {
	GlobalSSHPublicKey string                `json:"global_ssh_public_key"`
	DefaultSSHPullUser string                `json:"default_ssh_pull_user"`
	AskToCopyPublicKey bool                  `json:"ask_to_copy_public_key"`
	SMTPSettings       services.SMTPSettings `json:"smtp_settings"`
	EmailFrequency     string                `json:"email_frequency"`
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

		defaultUser, err := settingsService.GetDefaultSSHPullUser(r.Context())
		if err != nil {
			webutil.LogError(r, "get default ssh pull user failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to retrieve default SSH pull user", nil)
			return
		}

		askToCopyPublicKey, err := settingsService.GetAskToCopyPublicKey(r.Context())
		if err != nil {
			webutil.LogError(r, "get ask to copy public key failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to retrieve ask to copy public key", nil)
			return
		}

		smtpSettings, err := settingsService.GetSMTPSettings(r.Context())
		if err != nil {
			webutil.LogError(r, "get smtp settings failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to retrieve smtp settings", nil)
			return
		}
		// Redact password for API response
		smtpSettings.Password = ""

		emailFreq, err := settingsService.GetEmailFrequency(r.Context())
		if err != nil {
			webutil.LogError(r, "get email frequency failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to retrieve email frequency", nil)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, getSettingsResponse{
			GlobalSSHPublicKey: globalKey.PublicKey,
			DefaultSSHPullUser: defaultUser,
			AskToCopyPublicKey: askToCopyPublicKey,
			SMTPSettings:       smtpSettings,
			EmailFrequency:     emailFreq,
		})
	}
}
