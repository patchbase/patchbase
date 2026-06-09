package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

type sshJobInfo struct {
	handle           rivertype.PeriodicJobHandle
	frequencyMinutes int32
}

type dailyHourSchedule struct {
	hour int
}

func (s dailyHourSchedule) Next(current time.Time) time.Time {
	next := time.Date(current.Year(), current.Month(), current.Day(), s.hour, 0, 0, 0, time.UTC)
	if !next.After(current) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

type PeriodicJobManager struct {
	injector do.Injector
	queries  sql.Querier
	config   config.Config
	logger   *slog.Logger

	mu       sync.Mutex
	syncJobs map[string]rivertype.PeriodicJobHandle
	sshJobs  map[string]sshJobInfo
}

func NewPeriodicJobManager(i do.Injector) (services.PeriodicJobManager, error) {
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}
	cfg, err := do.Invoke[config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get config.Config: %w", err)
	}
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *slog.Logger: %w", err)
	}

	return &PeriodicJobManager{
		injector: i,
		queries:  queries,
		config:   cfg,
		logger:   logger.With("source", "queue.PeriodicJobManager"),
		syncJobs: make(map[string]rivertype.PeriodicJobHandle),
		sshJobs:  make(map[string]sshJobInfo),
		mu:       sync.Mutex{},
	}, nil
}

func (p *PeriodicJobManager) Initialize(ctx context.Context) error {
	p.logger.InfoContext(ctx, "initializing periodic jobs from database")

	// 1. Initialize Advisory Sync Jobs
	scopes, err := p.queries.ListAdvisoryScopes(ctx)
	if err != nil {
		return fmt.Errorf("list advisory scopes: %w", err)
	}
	for _, scope := range scopes {
		if err := p.AddAdvisorySyncJob(ctx, scope.ScopeKey, false); err != nil {
			p.logger.ErrorContext(ctx, "failed to register initial advisory sync job", "scope_key", scope.ScopeKey, "error", err)
		}
	}

	// 2. Initialize SSH Pull Jobs
	hosts, err := p.queries.ListApprovedSSHHosts(ctx)
	if err != nil {
		return fmt.Errorf("list approved ssh hosts: %w", err)
	}
	for _, host := range hosts {
		freq := int32(360) // default pull frequency in minutes
		if host.PullFrequencyMinutes != nil && *host.PullFrequencyMinutes > 0 {
			freq = *host.PullFrequencyMinutes
		}
		if err := p.AddSSHPullJob(ctx, host.ID, freq, false); err != nil {
			p.logger.ErrorContext(ctx, "failed to register initial ssh pull job", "host_id", host.ID, "error", err)
		}
	}

	// 3. Initialize Email Report Job
	settingsService, err := do.Invoke[services.Settings](p.injector)
	if err == nil {
		freq, _ := settingsService.GetEmailFrequency(ctx)
		if err := p.SetEmailReportJob(ctx, freq); err != nil {
			p.logger.ErrorContext(ctx, "failed to register email report job", "error", err)
		}
	} else {
		p.logger.ErrorContext(ctx, "failed to get settings service for email report init", "error", err)
	}

	return nil
}

func (p *PeriodicJobManager) AddAdvisorySyncJob(ctx context.Context, scopeKey string, runOnStart bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If already registered, skip adding again to avoid triggering immediate RunOnStart enqueue.
	if _, ok := p.syncJobs[scopeKey]; ok {
		return nil
	}

	client, err := do.Invoke[*river.Client[pgx.Tx]](p.injector)
	if err != nil {
		return fmt.Errorf("failed to get *river.Client: %w", err)
	}

	job := river.NewPeriodicJob(
		river.PeriodicInterval(p.config.AdvisorySync.RefreshInterval),
		func() (river.JobArgs, *river.InsertOpts) {
			return services.AdvisorySyncArgs{ScopeKey: scopeKey}, &river.InsertOpts{
				UniqueOpts: river.UniqueOpts{
					ByArgs: true,
					ByState: []rivertype.JobState{
						rivertype.JobStateAvailable,
						rivertype.JobStatePending,
						rivertype.JobStateRunning,
						rivertype.JobStateScheduled,
					},
				},
			}
		},
		&river.PeriodicJobOpts{
			ID:         "advisory-sync-" + scopeKey,
			RunOnStart: runOnStart,
		},
	)

	handle, err := client.PeriodicJobs().AddSafely(job)
	if err != nil {
		return fmt.Errorf("add periodic job: %w", err)
	}

	p.syncJobs[scopeKey] = handle
	p.logger.InfoContext(ctx, "registered periodic advisory sync job", "scope_key", scopeKey, "interval", p.config.AdvisorySync.RefreshInterval)
	return nil
}

func (p *PeriodicJobManager) RemoveAdvisorySyncJob(ctx context.Context, scopeKey string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, err := do.Invoke[*river.Client[pgx.Tx]](p.injector)
	if err != nil {
		return fmt.Errorf("failed to get *river.Client: %w", err)
	}

	if handle, ok := p.syncJobs[scopeKey]; ok {
		client.PeriodicJobs().Remove(handle)
		delete(p.syncJobs, scopeKey)
		p.logger.InfoContext(ctx, "removed periodic advisory sync job", "scope_key", scopeKey)
	} else {
		client.PeriodicJobs().RemoveByID("advisory-sync-" + scopeKey)
	}
	return nil
}

func (p *PeriodicJobManager) AddSSHPullJob(ctx context.Context, hostID string, frequencyMinutes int32, runOnStart bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, err := do.Invoke[*river.Client[pgx.Tx]](p.injector)
	if err != nil {
		return fmt.Errorf("failed to get *river.Client: %w", err)
	}

	// If already registered with same frequency, do nothing to avoid triggering RunOnStart.
	if info, ok := p.sshJobs[hostID]; ok {
		if info.frequencyMinutes == frequencyMinutes {
			return nil
		}
		client.PeriodicJobs().Remove(info.handle)
		delete(p.sshJobs, hostID)
	}

	interval := time.Duration(frequencyMinutes) * time.Minute
	job := river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return services.SSHPullArgs{HostID: hostID}, &river.InsertOpts{
				UniqueOpts: river.UniqueOpts{
					ByArgs: true,
					ByState: []rivertype.JobState{
						rivertype.JobStateAvailable,
						rivertype.JobStatePending,
						rivertype.JobStateRunning,
						rivertype.JobStateScheduled,
					},
				},
			}
		},
		&river.PeriodicJobOpts{
			ID:         "ssh-pull-" + hostID,
			RunOnStart: runOnStart,
		},
	)

	handle, err := client.PeriodicJobs().AddSafely(job)
	if err != nil {
		return fmt.Errorf("add periodic job: %w", err)
	}

	p.sshJobs[hostID] = sshJobInfo{
		handle:           handle,
		frequencyMinutes: frequencyMinutes,
	}
	p.logger.InfoContext(ctx, "registered periodic ssh pull job", "host_id", hostID, "interval", interval)
	return nil
}

func (p *PeriodicJobManager) RemoveSSHPullJob(ctx context.Context, hostID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, err := do.Invoke[*river.Client[pgx.Tx]](p.injector)
	if err != nil {
		return fmt.Errorf("failed to get *river.Client: %w", err)
	}

	if info, ok := p.sshJobs[hostID]; ok {
		client.PeriodicJobs().Remove(info.handle)
		delete(p.sshJobs, hostID)
		p.logger.InfoContext(ctx, "removed periodic ssh pull job", "host_id", hostID)
	} else {
		client.PeriodicJobs().RemoveByID("ssh-pull-" + hostID)
	}
	return nil
}

func (p *PeriodicJobManager) GetSyncJobsCountForTest() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.syncJobs)
}

func (p *PeriodicJobManager) HasSyncJobForTest(scopeKey string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.syncJobs[scopeKey]
	return ok
}

func (p *PeriodicJobManager) GetSSHJobsCountForTest() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sshJobs)
}

func (p *PeriodicJobManager) HasSSHJobForTest(hostID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.sshJobs[hostID]
	return ok
}

func (p *PeriodicJobManager) SetEmailReportJob(ctx context.Context, frequency string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, err := do.Invoke[*river.Client[pgx.Tx]](p.injector)
	if err != nil {
		return fmt.Errorf("failed to get *river.Client: %w", err)
	}

	client.PeriodicJobs().RemoveByID("email-report")

	if frequency != string(services.EmailFrequencyDaily) {
		p.logger.InfoContext(ctx, "email report job disabled or unknown frequency", "frequency", frequency)
		return nil
	}

	settingsSvc, err := do.Invoke[services.Settings](p.injector)
	if err != nil {
		return fmt.Errorf("failed to get settings service: %w", err)
	}
	smtpSettings, err := settingsSvc.GetSMTPSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get smtp settings: %w", err)
	}

	job := river.NewPeriodicJob(
		dailyHourSchedule{hour: smtpSettings.ReportHour},
		func() (river.JobArgs, *river.InsertOpts) {
			return SendReportArgs{}, &river.InsertOpts{
				UniqueOpts: river.UniqueOpts{
					ByArgs: true,
					ByState: []rivertype.JobState{
						rivertype.JobStateAvailable,
						rivertype.JobStatePending,
						rivertype.JobStateRunning,
						rivertype.JobStateScheduled,
					},
				},
			}
		},
		&river.PeriodicJobOpts{
			ID:         "email-report",
			RunOnStart: false,
		},
	)

	_, err = client.PeriodicJobs().AddSafely(job)
	if err != nil {
		return fmt.Errorf("add periodic email report job: %w", err)
	}

	p.logger.InfoContext(ctx, "registered periodic email report job", "frequency", frequency)
	return nil
}
