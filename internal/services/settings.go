package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	"go.patchbase.net/server/internal/utils"
)

const (
	InitialSetupDoneKey = "initial_setup_done"
	bootstrapAdminEmail = "admin@patchbase.local"
	bootstrapAdminName  = "Administrator"
)

type CompleteInitialSetupInput struct {
	Name     string
	Email    string
	Password string
}

type Settings interface {
	TryInitialSetup(ctx context.Context) (bool, error)
	Status(ctx context.Context) (InitialSetupDone, error)
	CompleteInitialSetup(ctx context.Context, userID string, input CompleteInitialSetupInput) (sql.User, error)
}

type settings struct {
	pool   *pgxpool.Pool
	sql    sql.Querier
	random utils.RandomStringGenerator
}

func NewSettings(i do.Injector) (Settings, error) {
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *pgxpool.Pool: %w", err)
	}
	sql, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}
	random, err := do.Invoke[utils.RandomStringGenerator](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get utils.RandomStringGenerator: %w", err)
	}
	return &settings{
		pool:   pool,
		sql:    sql,
		random: random,
	}, nil
}

func (s *settings) TryInitialSetup(ctx context.Context) (bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin initial setup transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := sql.New(tx)
	initialSetup := NewSettingManager[InitialSetupDone](InitialSetupDoneKey, queries)

	data, err := initialSetup.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get initial setup done setting: %w", err)
	}
	if data.Done {
		return false, nil
	}
	if err := initialSetup.Ensure(ctx, InitialSetupDone{Done: false}); err != nil {
		return false, fmt.Errorf("failed to ensure initial setup done setting exists: %w", err)
	}

	logger := utils.GetLogger(ctx).With("source", "settings.TryInitialSetup")
	admin, err := queries.GetAdminUser(ctx)
	if err == nil {
		logger.Info("initial setup pending, bootstrap admin already exists", "email", admin.Email)
		return false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("failed to get bootstrap admin user: %w", err)
	}

	password := s.random.Hex(16)
	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		return false, fmt.Errorf("failed to hash bootstrap admin password: %w", err)
	}

	admin, err = queries.CreateAdminUser(ctx, sql.CreateAdminUserParams{
		ID:           id.New("u"),
		Email:        bootstrapAdminEmail,
		Name:         bootstrapAdminName,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if sql.IsUniqueViolation(err, "users_email_active_unique_idx") {
			logger.Info("bootstrap admin creation raced with another instance")
			return false, nil
		}
		return false, fmt.Errorf("failed to create bootstrap admin user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit initial setup transaction: %w", err)
	}

	logBootstrapAdminCredentials(logger, admin.Email, password)
	return true, nil
}

func (s *settings) Status(ctx context.Context) (InitialSetupDone, error) {
	initialSetup := NewSettingManager[InitialSetupDone](InitialSetupDoneKey, s.sql)
	data, err := initialSetup.Get(ctx)
	if err != nil {
		return InitialSetupDone{}, fmt.Errorf("get initial setup status: %w", err)
	}

	return data, nil
}

func (s *settings) CompleteInitialSetup(ctx context.Context, userID string, input CompleteInitialSetupInput) (sql.User, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return sql.User{}, fmt.Errorf("begin initial setup completion transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := sql.New(tx)
	initialSetup := NewSettingManager[InitialSetupDone](InitialSetupDoneKey, queries)

	status, err := initialSetup.Get(ctx)
	if err != nil {
		return sql.User{}, fmt.Errorf("get initial setup status: %w", err)
	}
	if status.Done {
		return sql.User{}, ErrInitialSetupAlreadyComplete
	}

	name := strings.TrimSpace(input.Name)
	email := normalizeEmail(input.Email)
	if name == "" {
		return sql.User{}, fmt.Errorf("name is required")
	}
	if email == "" {
		return sql.User{}, fmt.Errorf("email is required")
	}
	if len(input.Password) < 12 {
		return sql.User{}, fmt.Errorf("password must be at least 12 characters")
	}

	passwordHash, err := utils.HashPassword(input.Password)
	if err != nil {
		return sql.User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := queries.CompleteInitialSetupForUser(ctx, sql.CompleteInitialSetupForUserParams{
		ID:           userID,
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if sql.IsUniqueViolation(err, "users_email_active_unique_idx") {
			return sql.User{}, ErrEmailAlreadyInUse
		}
		return sql.User{}, fmt.Errorf("update bootstrap admin credentials: %w", err)
	}

	if err := initialSetup.Set(ctx, InitialSetupDone{Done: true}); err != nil {
		return sql.User{}, fmt.Errorf("mark initial setup complete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sql.User{}, fmt.Errorf("commit initial setup completion transaction: %w", err)
	}

	return user, nil
}

type InitialSetupDone struct {
	Done bool `json:"done"`
}

type SettingManager[T any] interface {
	Get(ctx context.Context) (T, error)
	Ensure(ctx context.Context, value T) error
	Set(ctx context.Context, value T) error
}

type settingManager[T any] struct {
	sql sql.Querier
	key string
}

func NewSettingManager[T any](key string, sql sql.Querier) SettingManager[T] {
	return &settingManager[T]{
		sql: sql,
		key: key,
	}
}

func (m *settingManager[T]) Get(ctx context.Context) (T, error) {
	var zero T
	data, err := m.sql.GetSetting(ctx, m.key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, nil
		}
		return zero, fmt.Errorf("failed to get setting: %w", err)
	}
	var value T
	err = json.Unmarshal(data.Value, &value)
	if err != nil {
		return zero, fmt.Errorf("failed to unmarshal setting value: %w", err)
	}
	return value, nil
}

func (m *settingManager[T]) Set(ctx context.Context, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal setting value: %w", err)
	}
	_, err = m.sql.UpsertSetting(ctx, sql.UpsertSettingParams{
		Key:   m.key,
		Value: data,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert setting: %w", err)
	}
	return nil
}

func (m *settingManager[T]) Ensure(ctx context.Context, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal setting value: %w", err)
	}
	err = m.sql.CreateSettingIfAbsent(ctx, sql.CreateSettingIfAbsentParams{
		Key:   m.key,
		Value: data,
	})
	if err != nil {
		return fmt.Errorf("failed to create setting if absent: %w", err)
	}
	return nil
}

func logBootstrapAdminCredentials(logger *slog.Logger, email string, password string) {
	block := fmt.Sprintf(
		"\n==================== PatchBase initial setup ====================\n"+
			"bootstrap admin created\n"+
			"login url : /login\n"+
			"email     : %s\n"+
			"password  : %s\n"+
			"next step : sign in, then complete /setup to set permanent credentials\n"+
			"===============================================================\n",
		email,
		password,
	)

	_, _ = fmt.Fprint(os.Stderr, block)
	logger.Warn("bootstrap admin credentials emitted to stderr for initial setup")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
