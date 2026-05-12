package setup

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
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
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can complete setup", nil)
			return
		}

		var req completeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", nil)
			return
		}

		updatedUser, err := settings.CompleteInitialSetup(r.Context(), authInfo.User.ID, services.CompleteInitialSetupInput{
			Name:     req.Name,
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			switch {
			case errors.Is(err, services.ErrInitialSetupAlreadyComplete):
				webutil.WriteAPIError(w, r, http.StatusConflict, "initial setup already completed", nil)
			case errors.Is(err, services.ErrEmailAlreadyInUse):
				webutil.WriteAPIError(w, r, http.StatusConflict, "email is already in use", nil)
			default:
				webutil.LogError(r, "complete setup failed", err)
				webutil.WriteAPIError(w, r, http.StatusBadRequest, "complete setup failed", nil)
			}
			return
		}

		accessToken, err := authService.IssueAccessToken(r.Context(), updatedUser.ID)
		if err != nil {
			webutil.LogError(r, "issue refreshed setup token failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to issue access token", nil)
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
