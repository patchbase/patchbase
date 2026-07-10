package settings

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/mailer"
)

func SendReport(i do.Injector) apiauth.AuthenticatedHandler {
	emailService := do.MustInvoke[mailer.Mailer](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenSendReport)
			return
		}

		if err := emailService.SendReport(r.Context(), []string{authInfo.User.Email}); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}