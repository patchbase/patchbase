package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/migrations"
)

func runMigrate(cmd *cobra.Command, _ []string) error {
	logger := slog.Default().With("source", "cli.migrate")

	dbURL, err := cmd.Flags().GetString("database-url")
	if err != nil {
		return fmt.Errorf("failed to get database-url flag: %w", err)
	}
	if dbURL == "" {
		cfg, err := config.New()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		dbURL = cfg.Database.URL
	}

	db, err := migrations.Open(dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close() //nolint:errcheck

	logger.Info("running database migrations")
	if err := migrations.Up(cmd.Context(), db); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	logger.Info("database migrations complete")
	return nil
}

func NewMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending database migrations",
		Long:  "Apply pending database migrations embedded in this binary. Safe to run repeatedly; a no-op if the DB is up to date.",
		RunE:  runMigrate,
	}
	cmd.Flags().String("database-url", "", "override config.database.url")
	return cmd
}
