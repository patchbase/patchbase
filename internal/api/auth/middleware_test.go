// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/mock"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"go.uber.org/mock/gomock"
)

func setupMiddleware(t *testing.T, authMock func(ctx context.Context, token string) (sql.User, error), next auth.AuthenticatedHandler) http.HandlerFunc {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockAuth := mock.NewMockAuth(ctrl)
	if authMock != nil {
		mockAuth.EXPECT().Authenticate(gomock.Any(), gomock.Any()).DoAndReturn(authMock).AnyTimes()
	}

	i := do.New()
	do.ProvideValue[services.Auth](i, mockAuth)

	authMiddleware, err := auth.New(i)
	require.NoError(t, err)

	return authMiddleware.Required(next)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	nextCalled := false
	handler := setupMiddleware(t, nil, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"code":"missing_bearer_token","message":"missing bearer token"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_MalformedHeaderNoBearer(t *testing.T) {
	nextCalled := false
	handler := setupMiddleware(t, nil, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic abc")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"code":"missing_bearer_token","message":"missing bearer token"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_MalformedHeaderJustBearer(t *testing.T) {
	nextCalled := false
	handler := setupMiddleware(t, nil, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"code":"missing_bearer_token","message":"missing bearer token"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	nextCalled := false
	authMock := func(_ context.Context, _ string) (sql.User, error) {
		return sql.User{}, apperr.ErrUnauthorized
	}
	handler := setupMiddleware(t, authMock, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"code":"unauthorized","message":"invalid access token"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_InternalError(t *testing.T) {
	nextCalled := false
	authMock := func(_ context.Context, _ string) (sql.User, error) {
		return sql.User{}, errors.New("database connection failed")
	}
	handler := setupMiddleware(t, authMock, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token-but-db-fails")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"code":"internal_error","message":"internal server error"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	nextCalled := false
	authMock := func(_ context.Context, token string) (sql.User, error) {
		assert.Equal(t, "valid-token", token)
		return sql.User{ID: "user-123"}, nil
	}
	handler := setupMiddleware(t, authMock, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
		assert.Equal(t, "user-123", authInfo.User.ID)
		assert.Equal(t, "valid-token", authInfo.Token)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, nextCalled)
}
