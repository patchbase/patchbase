// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/migrations"
)

func runMigrate(cmd *cobra.Command, _ []string) error {
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

	return runMigrateWithURL(cmd.Context(), dbURL)
}

func runMigrateWithURL(ctx context.Context, dbURL string) error {
	logger := slog.Default().With("source", "cli.migrate")
	db, err := migrations.Open(dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close() //nolint:errcheck

	logger.Info("running database migrations")
	if err := migrations.Up(ctx, db); err != nil {
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
