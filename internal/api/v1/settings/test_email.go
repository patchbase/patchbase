// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package settings

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
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
			webutil.WriteError(w, r, apperr.ErrForbiddenTestEmail)
			return
		}

		var req testEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidBody, err))
			return
		}

		req.To = strings.TrimSpace(req.To)
		if req.To == "" {
			webutil.WriteError(w, r, apperr.ErrToAddressRequired)
			return
		}

		smtpSettings, err := settingsService.GetSMTPSettings(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		if err := emailService.TestConnection(r.Context(), smtpSettings, req.To); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
