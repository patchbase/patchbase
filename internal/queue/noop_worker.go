package queue

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
	"go.patchbase.net/server/internal/sql"
)

type NoopArgs struct{}

func (NoopArgs) Kind() string {
	return "noop"
}

type NoopWorker struct {
	river.WorkerDefaults[NoopArgs]

	logger *slog.Logger
}

func NewNoopWorker(_ sql.Querier, logger *slog.Logger) *NoopWorker {
	return &NoopWorker{
		logger: logger.With("source", "queue.NoopWorker"),
	}
}

func (w *NoopWorker) Work(ctx context.Context, _ *river.Job[NoopArgs]) error {
	w.logger.DebugContext(ctx, "noop job processed")
	return nil
}
