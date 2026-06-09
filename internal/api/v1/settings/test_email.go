package settings

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/mailer"
	"go.patchbase.net/server/internal/services"
)

type testEmailRequest struct {
	To string `json:"to"`
}

func TestEmail(i do.Injector) apiauth.AuthenticatedHandler {
	emailService := do.MustInvoke[mailer.Mailer](i)
	settingsService := do.MustInvoke[services.Settings](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can test email", nil)
			return
		}

		var req testEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", err)
			return
		}

		req.To = strings.TrimSpace(req.To)
		if req.To == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "to address is required", nil)
			return
		}

		smtpSettings, err := settingsService.GetSMTPSettings(r.Context())
		if err != nil {
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get smtp settings", err)
			return
		}

		if err := emailService.TestConnection(r.Context(), smtpSettings, req.To); err != nil {
			webutil.LogError(r, "failed to send test email", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to send test email", err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
