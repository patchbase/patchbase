package settings

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

type updateSettingsRequest struct {
	DefaultSSHPullUser utils.Option[string]                `json:"default_ssh_pull_user"`
	AskToCopyPublicKey utils.Option[bool]                  `json:"ask_to_copy_public_key"`
	SMTPSettings       utils.Option[services.SMTPSettings] `json:"smtp_settings"`
	EmailFrequency     utils.Option[string]                `json:"email_frequency"`
}

func UpdateSettings(i do.Injector) apiauth.AuthenticatedHandler {
	settingsService := do.MustInvoke[services.Settings](i)
	periodicManager := do.MustInvoke[services.PeriodicJobManager](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenAccessSettings)
			return
		}

		var req updateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidBody, err))
			return
		}

		if req.DefaultSSHPullUser.IsPresent() {
			user := strings.TrimSpace(req.DefaultSSHPullUser.Unwrap())
			if user == "" {
				webutil.WriteError(w, r, apperr.ErrDefaultSSHPullUserEmpty)
				return
			}
			if err := settingsService.SetDefaultSSHPullUser(r.Context(), user); err != nil {
				webutil.WriteError(w, r, err)
				return
			}
		}

		if req.AskToCopyPublicKey.IsPresent() {
			if err := settingsService.SetAskToCopyPublicKey(r.Context(), req.AskToCopyPublicKey.Unwrap()); err != nil {
				webutil.WriteError(w, r, err)
				return
			}
		}

		if req.SMTPSettings.IsPresent() {
			if err := settingsService.SetSMTPSettings(r.Context(), req.SMTPSettings.Unwrap()); err != nil {
				webutil.WriteError(w, r, err)
				return
			}
			freq, err := settingsService.GetEmailFrequency(r.Context())
			if err != nil {
				webutil.WriteError(w, r, err)
				return
			}
			if err := periodicManager.SetEmailReportJob(r.Context(), freq); err != nil {
				webutil.WriteError(w, r, err)
				return
			}
		}

		if req.EmailFrequency.IsPresent() {
			freq := strings.TrimSpace(req.EmailFrequency.Unwrap())
			if err := settingsService.SetEmailFrequency(r.Context(), freq); err != nil {
				webutil.WriteError(w, r, err)
				return
			}
			if err := periodicManager.SetEmailReportJob(r.Context(), freq); err != nil {
				webutil.WriteError(w, r, err)
				return
			}
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
