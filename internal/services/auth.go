package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

const accessTokenTTL = 24 * time.Hour

var (
	ErrInvalidCredentials          = errors.New("invalid credentials")
	ErrUnauthorized                = errors.New("unauthorized")
	ErrInitialSetupAlreadyComplete = errors.New("initial setup already complete")
	ErrEmailAlreadyInUse           = errors.New("email already in use")
)

type Auth interface {
	Login(ctx context.Context, email string, password string) (LoginResult, error)
	Authenticate(ctx context.Context, token string) (sql.User, error)
	IssueAccessToken(ctx context.Context, userID string) (string, error)
}

type LoginResult struct {
	AccessToken string
	User        sql.User
}

type auth struct {
	sql sql.Querier
}

func NewAuth(i do.Injector) (Auth, error) {
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}

	return &auth{sql: queries}, nil
}

func (a *auth) Login(ctx context.Context, email string, password string) (LoginResult, error) {
	user, err := a.sql.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, fmt.Errorf("get user by email: %w", err)
	}

	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		return LoginResult{}, ErrInvalidCredentials
	}

	token, err := signAccessToken(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("sign access token: %w", err)
	}

	return LoginResult{
		AccessToken: token,
		User:        user,
	}, nil
}

func (a *auth) Authenticate(ctx context.Context, token string) (sql.User, error) {
	userID, err := parseAccessTokenSubject(token)
	if err != nil {
		return sql.User{}, ErrUnauthorized
	}

	user, err := a.sql.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // FIXME
			return sql.User{}, ErrUnauthorized
		}
		return sql.User{}, fmt.Errorf("get user by id: %w", err)
	}

	if !verifyAccessToken(token, user) {
		return sql.User{}, ErrUnauthorized
	}

	return user, nil
}

func (a *auth) IssueAccessToken(ctx context.Context, userID string) (string, error) {
	user, err := a.sql.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user by id: %w", err)
	}

	token, err := signAccessToken(user)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}

	return token, nil
}

func signAccessToken(user sql.User) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
	})

	signed, err := token.SignedString([]byte(user.PasswordHash))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signed, nil
}

func verifyAccessToken(token string, user sql.User) bool {
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(parsed *jwt.Token) (any, error) {
		if parsed.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", parsed.Method.Alg())
		}
		return []byte(user.PasswordHash), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !parsed.Valid {
		return false
	}

	return claims.Subject == user.ID
}

func parseAccessTokenSubject(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	parser := jwt.NewParser()
	if _, _, err := parser.ParseUnverified(token, claims); err != nil {
		return "", fmt.Errorf("parse jwt without verification: %w", err)
	}
	if claims.Subject == "" {
		return "", fmt.Errorf("missing jwt subject")
	}

	return claims.Subject, nil
}
