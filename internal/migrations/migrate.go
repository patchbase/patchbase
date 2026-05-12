// Package migrations owns schema evolution for the PatchBase database.
// Migrations are embedded into the binary and applied via `patchbase-server migrate` before the server starts.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" sql driver
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed files/*.sql
var embeddedFS embed.FS

const (
	dialect       = "postgres"
	migrationsDir = "files"
)

// Up applies all pending migrations for PatchBase and River.
func Up(ctx context.Context, db *sql.DB) error {
	logger := slog.Default().With("source", "migrations.Up")

	goose.SetBaseFS(embeddedFS)
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}
	goose.SetLogger(gooseLogger{logger: logger})

	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	driver := riverdatabasesql.New(db)
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize river migrator: %w", err)
	}

	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("failed to apply river migrations: %w", err)
	}
	if len(res.Versions) == 0 {
		logger.Info("river migrations already up to date")
	} else {
		logger.Info("applied river migrations", "count", len(res.Versions))
	}

	return nil
}

// Open returns a *sql.DB backed by the pgx driver for the given Postgres URL.
// Callers are responsible for closing the returned DB.
func Open(url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open migration DB connection: %w", err)
	}
	return db, nil
}

// gooseLogger adapts slog to goose's Logger interface.
type gooseLogger struct {
	logger *slog.Logger
}

func (g gooseLogger) Fatalf(format string, v ...any) {
	g.logger.Error(fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (g gooseLogger) Printf(format string, v ...any) {
	g.logger.Info(fmt.Sprintf(format, v...))
}
