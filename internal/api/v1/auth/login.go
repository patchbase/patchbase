package auth

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken         string       `json:"access_token"`
	SetupCompleted      bool         `json:"setup_completed"`
	PasswordResetNeeded bool         `json:"password_reset_needed"`
	User                responseUser `json:"user"`
}

type responseUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func Login(i do.Injector) http.HandlerFunc {
	authService := do.MustInvoke[services.Auth](i)
	settings := do.MustInvoke[services.Settings](i)

	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		result, err := authService.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		status, err := settings.Status(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, loginResponse{
			AccessToken:         result.AccessToken,
			SetupCompleted:      status.Done,
			PasswordResetNeeded: result.User.PasswordResetRequired,
			User: responseUser{
				ID:    result.User.ID,
				Email: result.User.Email,
				Name:  result.User.Name,
			},
		})
	}
}