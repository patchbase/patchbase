package setup

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

type completeRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Complete(i do.Injector) apiauth.AuthenticatedHandler {
	settings := do.MustInvoke[services.Settings](i)
	authService := do.MustInvoke[services.Auth](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenCompleteSetup)
			return
		}

		var req completeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		updatedUser, err := settings.CompleteInitialSetup(r.Context(), authInfo.User.ID, services.CompleteInitialSetupInput{
			Name:     req.Name,
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		accessToken, err := authService.IssueAccessToken(r.Context(), updatedUser.ID)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]any{
			"access_token":          accessToken,
			"setup_completed":       true,
			"password_reset_needed": false,
			"user": map[string]string{
				"id":    updatedUser.ID,
				"email": updatedUser.Email,
				"name":  updatedUser.Name,
			},
		})
	}
}
