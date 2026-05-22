package queue_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/queue"
	"go.patchbase.net/server/internal/services"
)

type mockAdvisorySyncService struct {
	services.AdvisorySyncService
	SyncScopeFunc func(ctx context.Context, scopeKey string) error
}

func (m *mockAdvisorySyncService) SyncScope(ctx context.Context, scopeKey string) error {
	return m.SyncScopeFunc(ctx, scopeKey)
}

func TestAdvisorySyncWorker_Work_Success(t *testing.T) {
	called := false
	mockSync := &mockAdvisorySyncService{
		SyncScopeFunc: func(ctx context.Context, scopeKey string) error {
			assert.Equal(t, "debian:bookworm-dsa", scopeKey)
			called = true
			return nil
		},
	}

	injector := do.New()
	do.ProvideValue[*slog.Logger](injector, slog.Default())
	do.ProvideValue[services.AdvisorySyncService](injector, mockSync)

	worker, err := queue.NewAdvisorySyncWorker(injector)
	require.NoError(t, err)

	job := &river.Job[services.AdvisorySyncArgs]{
		Args: services.AdvisorySyncArgs{ScopeKey: "debian:bookworm-dsa"},
	}

	err = worker.Work(context.Background(), job)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestAdvisorySyncWorker_Work_Failure(t *testing.T) {
	called := false
	mockSync := &mockAdvisorySyncService{
		SyncScopeFunc: func(ctx context.Context, scopeKey string) error {
			called = true
			return fmt.Errorf("sync error")
		},
	}

	injector := do.New()
	do.ProvideValue[*slog.Logger](injector, slog.Default())
	do.ProvideValue[services.AdvisorySyncService](injector, mockSync)

	worker, err := queue.NewAdvisorySyncWorker(injector)
	require.NoError(t, err)

	job := &river.Job[services.AdvisorySyncArgs]{
		Args: services.AdvisorySyncArgs{ScopeKey: "debian:bookworm-dsa"},
	}

	err = worker.Work(context.Background(), job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sync error")
	assert.True(t, called)
}
