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
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

type mockAuthService struct {
	authenticateFunc func(ctx context.Context, token string) (sql.User, error)
}

func (m *mockAuthService) Login(ctx context.Context, email string, password string) (services.LoginResult, error) {
	panic("not implemented")
}

func (m *mockAuthService) Authenticate(ctx context.Context, token string) (sql.User, error) {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(ctx, token)
	}
	return sql.User{}, nil
}

func (m *mockAuthService) IssueAccessToken(ctx context.Context, userID string) (string, error) {
	panic("not implemented")
}

func (m *mockAuthService) UpdateProfile(ctx context.Context, userID string, input services.UpdateProfileInput) (services.UpdateProfileResult, error) {
	panic("not implemented")
}

func setupMiddleware(t *testing.T, authMock func(ctx context.Context, token string) (sql.User, error), next auth.AuthenticatedHandler) http.HandlerFunc {
	t.Helper()
	i := do.New()
	mockService := &mockAuthService{authenticateFunc: authMock}
	do.ProvideValue[services.Auth](i, mockService)

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
	assert.JSONEq(t, `{"error":"missing bearer token"}`, rec.Body.String())
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
	assert.JSONEq(t, `{"error":"missing bearer token"}`, rec.Body.String())
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
	assert.JSONEq(t, `{"error":"missing bearer token"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	nextCalled := false
	authMock := func(ctx context.Context, token string) (sql.User, error) {
		return sql.User{}, services.ErrUnauthorized
	}
	handler := setupMiddleware(t, authMock, func(w http.ResponseWriter, r *http.Request, authInfo auth.AuthInfo) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"invalid access token"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_InternalError(t *testing.T) {
	nextCalled := false
	authMock := func(ctx context.Context, token string) (sql.User, error) {
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
	assert.JSONEq(t, `{"error":"internal server error"}`, rec.Body.String())
	assert.False(t, nextCalled)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	nextCalled := false
	authMock := func(ctx context.Context, token string) (sql.User, error) {
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
