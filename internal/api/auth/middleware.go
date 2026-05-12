package auth

import (
	"fmt"
	"net/http"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

type AuthInfo struct {
	Token string
	User  sql.User
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
			webutil.WriteAPIError(w, r, http.StatusUnauthorized, "missing bearer token", nil)
			return
		}

		user, err := a.authService.Authenticate(r.Context(), token)
		if err != nil {
			if err == services.ErrUnauthorized {
				webutil.WriteAPIError(w, r, http.StatusUnauthorized, "invalid access token", nil)
				return
			}

			webutil.LogError(r, "authenticate request failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "authentication failed", nil)
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
