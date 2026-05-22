package testing

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	gotesting "testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/di"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

const (
	DefaultTestDatabaseURL = "postgres://postgres:postgres@localhost:5433/patchbase_test?sslmode=disable"
	defaultJWTSecretKey    = "test-secret"
)

type InjectorOverride func(i do.Injector)
type Fixture func(ctx context.Context, backend *Backend) error

type options struct {
	databaseURL string
	overrides   []InjectorOverride
	fixtures    []Fixture
}

type Option func(*options)

func WithDatabaseURL(url string) Option {
	return func(opts *options) {
		opts.databaseURL = url
	}
}

func WithInjectorOverride(override InjectorOverride) Option {
	return func(opts *options) {
		opts.overrides = append(opts.overrides, override)
	}
}

func WithFixture(fixture Fixture) Option {
	return func(opts *options) {
		opts.fixtures = append(opts.fixtures, fixture)
	}
}

type Backend struct {
	injector do.Injector
	config   config.Config
}

func NewBackend(t *gotesting.T, opts ...Option) *Backend {
	t.Helper()

	options := options{
		databaseURL: os.Getenv("PATCHBASE_TEST_DATABASE_URL"),
	}
	if options.databaseURL == "" {
		options.databaseURL = DefaultTestDatabaseURL
	}
	for _, opt := range opts {
		opt(&options)
	}

	cfg := config.Config{
		SkipValidation: true,
		API: config.API{
			JWTSecretKey:      defaultJWTSecretKey,
			ListenAddress:     config.DefaultAPIListenAddress,
			Port:              config.DefaultAPIPort,
			ReadTimeout:       config.DefaultReadTimeout,
			ReadHeaderTimeout: config.DefaultReadHeaderTimeout,
			WriteTimeout:      config.DefaultWriteTimeout,
			ShutdownTimeout:   config.DefaultShutdownTimeout,
		},
		Database: config.Database{
			URL:      options.databaseURL,
			LogLevel: config.DefaultDatabaseLogLevel,
		},
		AdvisorySync: config.AdvisorySync{
			RefreshInterval: 1 * time.Hour,
		},
		EncryptionKey: "test-encryption-key-for-unit-tests",
	}

	testDBURL, err := createEphemeralDatabase(t, cfg.Database.URL)
	if err != nil {
		t.Skipf("skipping integration test: test database unavailable: %v", err)
	}
	cfg.Database.URL = testDBURL

	ctx := context.Background()
	logger := slog.Default().With("source", "internal/testing")
	ctx = utils.WithLogger(ctx, logger)

	injector := newInjector(ctx, cfg, options.overrides)

	for _, fixture := range options.fixtures {
		if err := fixture(ctx, &Backend{
			injector: injector,
			config:   cfg,
		}); err != nil {
			t.Fatalf("load fixture failed: %v", err)
		}
	}

	pool := do.MustInvoke[*pgxpool.Pool](injector)
	t.Cleanup(pool.Close)

	return &Backend{
		injector: injector,
		config:   cfg,
	}
}

func newInjector(ctx context.Context, cfg config.Config, overrides []InjectorOverride) do.Injector {
	injector := di.New(ctx, cfg)
	for _, override := range overrides {
		override(injector)
	}
	return injector
}

func (b *Backend) Injector() do.Injector {
	return b.injector
}

func (b *Backend) Config() config.Config {
	return b.config
}

func (b *Backend) IssueAccessToken(ctx context.Context, userID string) (string, error) {
	authService := do.MustInvoke[services.Auth](b.injector)
	token, err := authService.IssueAccessToken(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("issue access token: %w", err)
	}
	return token, nil
}
