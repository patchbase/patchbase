package queue

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/samber/do/v2"
)

const runtimeVerificationQueueWorkers = 4

func NewRiverClient(i do.Injector) (*river.Client[pgx.Tx], error) {
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *pgxpool.Pool: %w", err)
	}
	// queries, err := do.Invoke[sql.Querier](i)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	// }
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *slog.Logger: %w", err)
	}

	workers := river.NewWorkers()

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  logger.With("source", "river"),
		Workers: workers,
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
