package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/mailer"
)

type SendReportArgs struct{}

func (SendReportArgs) Kind() string { return "send_report" }

type EmailReportWorker struct {
	river.WorkerDefaults[SendReportArgs]
	mailer mailer.Mailer
}

func NewEmailReportWorker(i do.Injector) (*EmailReportWorker, error) {
	emailService, err := do.Invoke[mailer.Mailer](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke mailer.Mailer: %w", err)
	}
	return &EmailReportWorker{
		WorkerDefaults: river.WorkerDefaults[SendReportArgs]{},
		mailer:         emailService,
	}, nil
}

func (w *EmailReportWorker) Work(ctx context.Context, job *river.Job[SendReportArgs]) error {
	err := w.mailer.SendReport(ctx, nil)
	if err != nil {
		// Log it but perhaps don't retry if smtp is unconfigured?
		// river will retry by default if we return an error.
		return fmt.Errorf("failed to send report: %w", err)
	}
	return nil
}
