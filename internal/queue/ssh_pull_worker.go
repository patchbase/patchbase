package queue

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/services"
)

type SSHPullWorker struct {
	river.WorkerDefaults[services.SSHPullArgs]

	injector do.Injector
	logger   *slog.Logger
}

func NewSSHPullWorker(i do.Injector) (*SSHPullWorker, error) {
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, err
	}
	return &SSHPullWorker{
		injector: i,
		logger:   logger.With("source", "queue.SSHPullWorker"),
	}, nil
}

func (w *SSHPullWorker) NextRetry(job *river.Job[services.SSHPullArgs]) time.Time {
	// Base backoff of 10 seconds, doubling with each attempt, capped at 10 minutes.
	backoff := 10 * time.Second * (1 << uint(job.Attempt-1))
	if backoff > 10*time.Minute || backoff <= 0 {
		backoff = 10 * time.Minute
	}
	return time.Now().Add(backoff)
}

func (w *SSHPullWorker) Work(ctx context.Context, job *river.Job[services.SSHPullArgs]) error {
	hostsService, err := do.Invoke[services.Hosts](w.injector)
	if err != nil {
		return err
	}

	w.logger.InfoContext(ctx, "starting ssh pull job", "host_id", job.Args.HostID)

	if err := hostsService.RunSSHPull(ctx, job.Args.HostID); err != nil {
		w.logger.ErrorContext(ctx, "ssh pull job failed", "host_id", job.Args.HostID, "error", err)

		if errors.Is(err, services.ErrHostNotFound) {
			return river.JobCancel(err)
		}

		var sshErr *services.SSHPullError
		if errors.As(err, &sshErr) {
			if sshErr.ExitCode != -1 && sshErr.ExitCode != 255 {
				// Command executed but returned non-zero. Not retriable.
				return river.JobCancel(err)
			}
		}
		return err
	}

	w.logger.InfoContext(ctx, "ssh pull job completed successfully", "host_id", job.Args.HostID)
	return nil
}
