package queue

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/sql"
)

const runtimeVerificationQueueWorkers = 4

func NewRiverClient(i do.Injector) (*river.Client[pgx.Tx], error) {
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *pgxpool.Pool: %w", err)
	}
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *slog.Logger: %w", err)
	}
	cfg, err := do.Invoke[config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get config.Config: %w", err)
	}

	sshWorker, err := NewSSHPullWorker(i)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh pull worker: %w", err)
	}

	advisoryWorker, err := NewAdvisorySyncWorker(i)
	if err != nil {
		return nil, fmt.Errorf("failed to create advisory sync worker: %w", err)
	}

	emailWorker, err := NewEmailReportWorker(i)
	if err != nil {
		return nil, fmt.Errorf("failed to create email report worker: %w", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, NewNoopWorker(queries, logger))
	river.AddWorker(workers, sshWorker)
	river.AddWorker(workers, advisoryWorker)
	river.AddWorker(workers, emailWorker)

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:   logger.With("source", "river"),
		Workers:  workers,
		TestOnly: cfg.SkipValidation,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: runtimeVerificationQueueWorkers,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new river client: %w", err)
	}

	return client, nil
}
