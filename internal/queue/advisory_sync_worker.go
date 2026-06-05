package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/services"
)

type AdvisorySyncWorker struct {
	river.WorkerDefaults[services.AdvisorySyncArgs]

	injector do.Injector
	logger   *slog.Logger
}

const advisorySyncJobTimeout = 30 * time.Minute

func NewAdvisorySyncWorker(i do.Injector) (*AdvisorySyncWorker, error) {
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, err
	}
	return &AdvisorySyncWorker{
		injector: i,
		logger:   logger.With("source", "queue.AdvisorySyncWorker"),
	}, nil
}

func (w *AdvisorySyncWorker) NextRetry(job *river.Job[services.AdvisorySyncArgs]) time.Time {
	backoff := 10 * time.Second * (1 << uint(job.Attempt-1))
	if backoff > 10*time.Minute || backoff <= 0 {
		backoff = 10 * time.Minute
	}
	return time.Now().Add(backoff)
}

func (w *AdvisorySyncWorker) Timeout(_ *river.Job[services.AdvisorySyncArgs]) time.Duration {
	return advisorySyncJobTimeout
}

func (w *AdvisorySyncWorker) Work(ctx context.Context, job *river.Job[services.AdvisorySyncArgs]) error {
	advisoriesService, err := do.Invoke[services.AdvisorySyncService](w.injector)
	if err != nil {
		return err
	}

	w.logger.InfoContext(ctx, "starting advisory sync job", "scope_key", job.Args.ScopeKey)
	if err := advisoriesService.SyncScope(ctx, job.Args.ScopeKey); err != nil {
		w.logger.ErrorContext(ctx, "advisory sync job failed", "scope_key", job.Args.ScopeKey, "error", err)
		return err
	}

	w.logger.InfoContext(ctx, "advisory sync job completed successfully", "scope_key", job.Args.ScopeKey)
	return nil
}
