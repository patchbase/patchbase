package services

import "context"

type PeriodicJobManager interface {
	Initialize(ctx context.Context) error
	AddAdvisorySyncJob(ctx context.Context, scopeKey string) error
	RemoveAdvisorySyncJob(ctx context.Context, scopeKey string) error
	AddSSHPullJob(ctx context.Context, hostID string, frequencyMinutes int32) error
	RemoveSSHPullJob(ctx context.Context, hostID string) error
}
