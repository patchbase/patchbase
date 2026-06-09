package settings

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
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
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can access settings", nil)
			return
		}

		var req updateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", err)
			return
		}

		if req.DefaultSSHPullUser.IsPresent() {
			user := strings.TrimSpace(req.DefaultSSHPullUser.Unwrap())
			if user == "" {
				webutil.WriteAPIError(w, r, http.StatusBadRequest, "default ssh pull user cannot be empty", nil)
				return
			}
			if err := settingsService.SetDefaultSSHPullUser(r.Context(), user); err != nil {
				webutil.LogError(r, "set default ssh pull user failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update default ssh pull user", err)
				return
			}
		}

		if req.AskToCopyPublicKey.IsPresent() {
			if err := settingsService.SetAskToCopyPublicKey(r.Context(), req.AskToCopyPublicKey.Unwrap()); err != nil {
				webutil.LogError(r, "set ask to copy public key failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update ask to copy public key", err)
				return
			}
		}

		if req.SMTPSettings.IsPresent() {
			if err := settingsService.SetSMTPSettings(r.Context(), req.SMTPSettings.Unwrap()); err != nil {
				webutil.LogError(r, "set smtp settings failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update smtp settings", err)
				return
			}
			freq, err := settingsService.GetEmailFrequency(r.Context())
			if err != nil {
				webutil.LogError(r, "get email frequency failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get email frequency", err)
				return
			}
			if err := periodicManager.SetEmailReportJob(r.Context(), freq); err != nil {
				webutil.LogError(r, "set email report job failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update email report job", err)
				return
			}
		}

		if req.EmailFrequency.IsPresent() {
			freq := strings.TrimSpace(req.EmailFrequency.Unwrap())
			if err := settingsService.SetEmailFrequency(r.Context(), freq); err != nil {
				webutil.LogError(r, "set email frequency failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update email frequency", err)
				return
			}
			if err := periodicManager.SetEmailReportJob(r.Context(), freq); err != nil {
				webutil.LogError(r, "set email report job failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update email report job", err)
				return
			}
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
