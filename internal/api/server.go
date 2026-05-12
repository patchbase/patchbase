package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

type Server struct {
	config config.Config
	logger *slog.Logger
	server *http.Server
	river  *river.Client[pgx.Tx]
}

func New(ctx context.Context, injector do.Injector) (*Server, error) {
	cfg := do.MustInvoke[config.Config](injector)
	logger := do.MustInvoke[*slog.Logger](injector).With("source", "WebServer")
	ids := do.MustInvoke[utils.RandomStringGenerator](injector)
	mux := do.MustInvoke[*http.ServeMux](injector)
	settings := do.MustInvoke[services.Settings](injector)
	riverClient := do.MustInvoke[*river.Client[pgx.Tx]](injector)

	if _, err := settings.TryInitialSetup(ctx); err != nil {
		return nil, fmt.Errorf("initial setup: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.API.ListenAddress, cfg.API.Port)

	return &Server{
		config: cfg,
		logger: logger,
		server: &http.Server{
			Addr:              addr,
			Handler:           SecurityHeadersMiddleware(RequestContextMiddleware(logger, ids, LoggingMiddleware(mux))),
			ReadTimeout:       cfg.API.ReadTimeout,
			WriteTimeout:      cfg.API.WriteTimeout,
			ReadHeaderTimeout: cfg.API.ReadHeaderTimeout,
		},
		river: riverClient,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.river.Start(ctx); err != nil {
		return fmt.Errorf("start river client: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), s.config.API.ShutdownTimeout)
		defer cancel()

		if err := s.river.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
			s.logger.ErrorContext(ctx, "river stop failed", "error", err)
		}
	}()

	errCh := make(chan error, 1)

	go func() {
		s.logger.InfoContext(ctx, "http server starting", "addr", s.server.Addr)
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.API.ShutdownTimeout)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}
