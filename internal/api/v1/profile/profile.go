package profile

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

type updateProfileRequest struct {
	Email           utils.Option[string] `json:"email"`
	CurrentPassword utils.Option[string] `json:"current_password"`
	NewPassword     utils.Option[string] `json:"new_password"`
}

type responseUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type profileResponse struct {
	AccessToken string       `json:"access_token"`
	User        responseUser `json:"user"`
}

func GetProfile(_ do.Injector) apiauth.AuthenticatedHandler {
	return func(w http.ResponseWriter, _ *http.Request, authInfo apiauth.AuthInfo) {
		webutil.WriteJSON(w, http.StatusOK, profileResponse{
			AccessToken: authInfo.Token,
			User:        toResponseUser(authInfo.User),
		})
	}
}

func UpdateProfile(i do.Injector) apiauth.AuthenticatedHandler {
	authService := do.MustInvoke[services.Auth](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		var req updateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", nil)
			return
		}

		result, err := authService.UpdateProfile(r.Context(), authInfo.User.ID, services.UpdateProfileInput{
			Email:           req.Email,
			CurrentPassword: req.CurrentPassword,
			NewPassword:     req.NewPassword,
		})
		if err != nil {
			switch {
			case errors.Is(err, services.ErrEmailRequired):
				webutil.WriteAPIError(w, r, http.StatusBadRequest, "email is required", nil)
			case errors.Is(err, services.ErrEmailAlreadyInUse):
				webutil.WriteAPIError(w, r, http.StatusConflict, "email is already in use", nil)
			case errors.Is(err, services.ErrCurrentPasswordRequired):
				webutil.WriteAPIError(w, r, http.StatusBadRequest, "current password is required", nil)
			case errors.Is(err, services.ErrCurrentPasswordInvalid):
				webutil.WriteAPIError(w, r, http.StatusUnauthorized, "current password is invalid", nil)
			case errors.Is(err, services.ErrPasswordTooShort):
				webutil.WriteAPIError(w, r, http.StatusBadRequest, "password must be at least 12 characters", nil)
			default:
				webutil.LogError(r, "update profile failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to update profile", nil)
			}
			return
		}

		accessToken := authInfo.Token
		if result.PasswordChanged {
			var err error
			accessToken, err = authService.IssueAccessToken(r.Context(), result.User.ID)
			if err != nil {
				webutil.LogError(r, "issue refreshed profile token failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to issue access token", nil)
				return
			}
		}

		webutil.WriteJSON(w, http.StatusOK, profileResponse{
			AccessToken: accessToken,
			User:        toResponseUser(result.User),
		})
	}
}

func toResponseUser(user sql.User) responseUser {
	return responseUser{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}
}
