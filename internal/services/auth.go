// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

const accessTokenTTL = 24 * time.Hour

type Auth interface {
	Login(ctx context.Context, email string, password string, ipAddress string, userAgent string) (LoginResult, error)
	Authenticate(ctx context.Context, token string) (sql.User, error)
	IssueAccessToken(ctx context.Context, userID string) (string, error)
	UpdateProfile(ctx context.Context, actor ActorRef, userID string, input UpdateProfileInput) (UpdateProfileResult, error)
}

type LoginResult struct {
	AccessToken string
	User        sql.User
}

type UpdateProfileInput struct {
	Email           utils.Option[string]
	CurrentPassword utils.Option[string]
	NewPassword     utils.Option[string]
}

type UpdateProfileResult struct {
	User            sql.User
	PasswordChanged bool
}

type auth struct {
	pool         *pgxpool.Pool
	sql          sql.Querier
	jwtSecretKey string
	audit        AuditLogService
}

func NewAuth(i do.Injector) (Auth, error) {
	cfg, err := do.Invoke[config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get config.Config: %w", err)
	}
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *pgxpool.Pool: %w", err)
	}
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}
	audit, err := do.Invoke[AuditLogService](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit log service: %w", err)
	}

	return &auth{
		pool:         pool,
		sql:          queries,
		jwtSecretKey: cfg.API.JWTSecretKey,
		audit:        audit,
	}, nil
}

func (a *auth) Login(ctx context.Context, email string, password string, ipAddress string, userAgent string) (LoginResult, error) {
	normalized := normalizeEmail(email)
	user, err := sql.Required(a.sql.GetUserByEmail(ctx, normalized))(apperr.ErrInvalidCredentials)
	if err != nil {
		a.recordLoginFailure(ctx, normalized, email, ipAddress, userAgent)
		return LoginResult{}, fmt.Errorf("get user by email: %w", err)
	}

	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		a.recordLoginFailure(ctx, normalized, email, ipAddress, userAgent)
		return LoginResult{}, apperr.ErrInvalidCredentials
	}

	token, err := signAccessToken(user, a.jwtSecretKey)
	if err != nil {
		return LoginResult{}, fmt.Errorf("sign access token: %w", err)
	}

	a.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    user.ID,
		ActorEmail: user.Email,
		Action:     auditLogActionLoginSuccess,
		TargetType: auditLogTargetTypeUser,
		TargetID:   user.ID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	return LoginResult{
		AccessToken: token,
		User:        user,
	}, nil
}

func (a *auth) recordLoginFailure(ctx context.Context, normalizedEmail string, rawEmail string, ipAddress string, userAgent string) {
	email := normalizedEmail
	if email == "" {
		email = rawEmail
	}
	a.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorEmail: email,
		Action:     auditLogActionLoginFailure,
		TargetType: auditLogTargetTypeUser,
		TargetID:   "",
		Metadata: map[string]any{
			"reason": "invalid_credentials",
		},
		IPAddress: ipAddress,
		UserAgent: userAgent,
	})
}

func (a *auth) Authenticate(ctx context.Context, token string) (sql.User, error) {
	userID, err := parseAccessTokenSubject(token)
	if err != nil {
		return sql.User{}, apperr.ErrUnauthorized
	}

	user, err := sql.Required(a.sql.GetUserByID(ctx, userID))(apperr.ErrUnauthorized)
	if err != nil {
		return sql.User{}, fmt.Errorf("get user by id: %w", err)
	}

	if !verifyAccessToken(token, user, a.jwtSecretKey) {
		return sql.User{}, apperr.ErrUnauthorized
	}

	return user, nil
}

func (a *auth) IssueAccessToken(ctx context.Context, userID string) (string, error) {
	user, err := a.sql.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user by id: %w", err)
	}

	token, err := signAccessToken(user, a.jwtSecretKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}

	return token, nil
}

func (a *auth) UpdateProfile(ctx context.Context, actor ActorRef, userID string, input UpdateProfileInput) (UpdateProfileResult, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return UpdateProfileResult{}, fmt.Errorf("begin profile update transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	user, err := sql.Required(queries.GetUserByID(ctx, userID))(apperr.ErrUnauthorized)
	if err != nil {
		return UpdateProfileResult{}, fmt.Errorf("get user by id: %w", err)
	}

	emailChanged := false
	if input.Email.IsPresent() {
		email := normalizeEmail(input.Email.Unwrap())
		if email == "" {
			return UpdateProfileResult{}, apperr.ErrEmailRequired
		}

		user, err = queries.UpdateUserEmail(ctx, sql.UpdateUserEmailParams{
			ID:    user.ID,
			Email: email,
		})
		if err != nil {
			if sql.IsUniqueViolation(err, "users_email_active_unique_idx") {
				return UpdateProfileResult{}, apperr.ErrEmailAlreadyInUse
			}
			return UpdateProfileResult{}, fmt.Errorf("update user email: %w", err)
		}
		emailChanged = true
	}

	passwordChanged := false
	if input.NewPassword.IsPresent() {
		newPassword := input.NewPassword.Unwrap()
		currentPassword, ok := input.CurrentPassword.Get()
		if !ok || currentPassword == "" {
			return UpdateProfileResult{}, apperr.ErrCurrentPasswordRequired
		}
		if !utils.CheckPasswordHash(currentPassword, user.PasswordHash) {
			return UpdateProfileResult{}, apperr.ErrCurrentPasswordInvalid
		}
		if len(newPassword) < 12 {
			return UpdateProfileResult{}, apperr.ErrPasswordTooShort
		}

		passwordHash, err := utils.HashPassword(newPassword)
		if err != nil {
			return UpdateProfileResult{}, fmt.Errorf("hash password: %w", err)
		}
		user, err = queries.UpdateUserPassword(ctx, sql.UpdateUserPasswordParams{
			ID:           user.ID,
			PasswordHash: passwordHash,
		})
		if err != nil {
			return UpdateProfileResult{}, fmt.Errorf("update user password: %w", err)
		}
		passwordChanged = true
	}

	if err := tx.Commit(ctx); err != nil {
		return UpdateProfileResult{}, fmt.Errorf("commit profile update transaction: %w", err)
	}

	if emailChanged || passwordChanged {
		metadata := map[string]any{}
		if emailChanged {
			metadata["email_changed"] = true
		}
		if passwordChanged {
			metadata["password_changed"] = true
		}
		a.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
			ActorID:    user.ID,
			ActorEmail: user.Email,
			Action:     auditLogActionProfileUpdate,
			TargetType: auditLogTargetTypeUser,
			TargetID:   user.ID,
			Metadata:   metadata,
			IPAddress:  actor.IP,
			UserAgent:  actor.UserAgent,
		})
	}

	return UpdateProfileResult{
		User:            user,
		PasswordChanged: passwordChanged,
	}, nil
}

func signAccessToken(user sql.User, jwtSecretKey string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
	})

	signed, err := token.SignedString(jwtSigningKey(jwtSecretKey, user))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signed, nil
}

func verifyAccessToken(token string, user sql.User, jwtSecretKey string) bool {
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(parsed *jwt.Token) (any, error) {
		if parsed.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", parsed.Method.Alg())
		}
		return jwtSigningKey(jwtSecretKey, user), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !parsed.Valid {
		return false
	}

	return claims.Subject == user.ID
}

func jwtSigningKey(jwtSecretKey string, user sql.User) []byte {
	mac := hmac.New(sha256.New, []byte(jwtSecretKey))
	_, _ = mac.Write([]byte(user.PasswordHash))
	return mac.Sum(nil)
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
