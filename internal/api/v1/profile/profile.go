// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package profile

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
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
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
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
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		result, err := authService.UpdateProfile(r.Context(), authInfo.ActorFromRequest(r), authInfo.User.ID, services.UpdateProfileInput{
			Email:           req.Email,
			CurrentPassword: req.CurrentPassword,
			NewPassword:     req.NewPassword,
		})
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		accessToken := authInfo.Token
		if result.PasswordChanged {
			var err error
			accessToken, err = authService.IssueAccessToken(r.Context(), result.User.ID)
			if err != nil {
				webutil.WriteError(w, r, err)
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
		ID:      user.ID,
		Email:   user.Email,
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
	}
}
