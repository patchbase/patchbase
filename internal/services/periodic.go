// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package services

import "context"

type PeriodicJobManager interface {
	Initialize(ctx context.Context) error
	AddAdvisorySyncJob(ctx context.Context, scopeKey string, runOnStart bool) error
	RemoveAdvisorySyncJob(ctx context.Context, scopeKey string) error
	AddSSHPullJob(ctx context.Context, hostID string, frequencyMinutes int32, runOnStart bool) error
	RemoveSSHPullJob(ctx context.Context, hostID string) error
	SetEmailReportJob(ctx context.Context, frequency string) error
}
