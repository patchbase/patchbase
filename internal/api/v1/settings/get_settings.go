package settings

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
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
			webutil.WriteError(w, r, apperr.ErrForbiddenAccessSettings)
			return
		}

		globalKey, err := settingsService.GetGlobalSSHKeyPair(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		defaultUser, err := settingsService.GetDefaultSSHPullUser(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		askToCopyPublicKey, err := settingsService.GetAskToCopyPublicKey(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		smtpSettings, err := settingsService.GetSMTPSettings(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}
		// Redact password for API response
		smtpSettings.Password = ""

		emailFreq, err := settingsService.GetEmailFrequency(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
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
