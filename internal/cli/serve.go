package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"go.patchbase.net/server/internal/api"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/di"
)

func runServe(cmd *cobra.Command, args []string) {
	cfg, err := config.New()
	if err != nil {
		slog.Default().Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	autoMigrate, _ := cmd.Flags().GetBool("automigrate")
	if autoMigrate {
		if err := runMigrateWithURL(cmd.Context(), cfg.Database.URL); err != nil {
			slog.Default().Error("automigrate failed", "error", err)
			os.Exit(1)
		}
	}

	injector := di.New(cmd.Context(), *cfg)
	server, err := api.New(cmd.Context(), injector)
	if err != nil {
		slog.Default().Error("Failed to create server", "error", err)
		os.Exit(1)
	}
	if err := server.Run(cmd.Context()); err != nil {
		slog.Default().Error("Server error", "error", err)
		os.Exit(1)
	}
	fmt.Println("Server stopped gracefully")
}

func NewServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Run:   runServe,
	}
	cmd.Flags().Bool("automigrate", false, "run database migrations before starting the server")
	return cmd
}
