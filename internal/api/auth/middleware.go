// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

type AuthInfo struct {
	Token string
	User  sql.User
}

// Actor returns a services.ActorRef derived from the authenticated user so
// that downstream service calls can record audit events with consistent
// identity, IP and email fields.
func (a AuthInfo) Actor() services.ActorRef {
	return services.ActorRef{ // nolint: exhaustruct
		UserID: a.User.ID,
		Email:  a.User.Email,
	}
}

// ActorFromRequest enriches the AuthInfo-derived actor with the request's IP address
// and user agent so audit log entries can trace which client performed the action.
func (a AuthInfo) ActorFromRequest(r *http.Request) services.ActorRef {
	actor := a.Actor()
	actor.IP = webutil.ClientIP(r)
	actor.UserAgent = r.UserAgent()
	return actor
}

type AuthenticatedHandler func(w http.ResponseWriter, r *http.Request, authInfo AuthInfo)

type Auth interface {
	Required(next AuthenticatedHandler) http.HandlerFunc
}

type auth struct {
	authService services.Auth
}

func New(i do.Injector) (Auth, error) {
	authService, err := do.Invoke[services.Auth](i)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Auth service: %w", err)
	}
	return &auth{
		authService: authService,
	}, nil
}

type AuthMiddleware func(next AuthenticatedHandler) http.HandlerFunc

func (a *auth) Required(next AuthenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			webutil.WriteError(w, r, apperr.ErrMissingBearer)
			return
		}

		user, err := a.authService.Authenticate(r.Context(), token)
		if err != nil {
			if errors.Is(err, apperr.ErrUnauthorized) {
				webutil.WriteError(w, r, apperr.ErrUnauthorized)
				return
			}
			webutil.WriteError(w, r, err)
			return
		}

		next(w, r, AuthInfo{
			Token: token,
			User:  user,
		})
	}
}

func bearerToken(authHeader string) (string, bool) {
	const prefix = "Bearer "
	if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
		return "", false
	}
	return authHeader[len(prefix):], true
}
