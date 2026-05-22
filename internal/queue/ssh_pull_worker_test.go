package queue_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/queue"
	"go.patchbase.net/server/internal/services"
)

type mockHosts struct {
	services.Hosts
	RunSSHPullFunc func(ctx context.Context, hostID string) error
}

func (m *mockHosts) RunSSHPull(ctx context.Context, hostID string) error {
	return m.RunSSHPullFunc(ctx, hostID)
}

func TestSSHPullWorker_Work_HostNotFound(t *testing.T) {
	injector := do.New()
	do.ProvideValue[*slog.Logger](injector, slog.Default())
	
	mock := &mockHosts{
		RunSSHPullFunc: func(ctx context.Context, hostID string) error {
			return services.ErrHostNotFound
		},
	}
	do.ProvideValue[services.Hosts](injector, mock)

	worker, err := queue.NewSSHPullWorker(injector)
	require.NoError(t, err)

	job := &river.Job[services.SSHPullArgs]{
		Args: services.SSHPullArgs{HostID: "test-host-id"},
	}

	err = worker.Work(context.Background(), job)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrHostNotFound)
	assert.Contains(t, err.Error(), "JobCancel")
}

func TestSSHPullWorker_Work_CommandErrorCancel(t *testing.T) {
	injector := do.New()
	do.ProvideValue[*slog.Logger](injector, slog.Default())
	
	sshErr := &services.SSHPullError{
		ExitCode: 1,
		Message:  "bash: command not found",
		Err:      errors.New("exit status 1"),
	}
	
	mock := &mockHosts{
		RunSSHPullFunc: func(ctx context.Context, hostID string) error {
			return sshErr
		},
	}
	do.ProvideValue[services.Hosts](injector, mock)

	worker, err := queue.NewSSHPullWorker(injector)
	require.NoError(t, err)

	job := &river.Job[services.SSHPullArgs]{
		Args: services.SSHPullArgs{HostID: "test-host-id"},
	}

	err = worker.Work(context.Background(), job)
	require.Error(t, err)
	assert.ErrorIs(t, err, sshErr)
	assert.Contains(t, err.Error(), "JobCancel")
}

func TestSSHPullWorker_Work_ConnectionErrorRetry(t *testing.T) {
	injector := do.New()
	do.ProvideValue[*slog.Logger](injector, slog.Default())
	
	sshErr := &services.SSHPullError{
		ExitCode: 255,
		Message:  "ssh: connect to host myhost port 22: Connection timed out",
		Err:      errors.New("exit status 255"),
	}
	
	mock := &mockHosts{
		RunSSHPullFunc: func(ctx context.Context, hostID string) error {
			return sshErr
		},
	}
	do.ProvideValue[services.Hosts](injector, mock)

	worker, err := queue.NewSSHPullWorker(injector)
	require.NoError(t, err)

	job := &river.Job[services.SSHPullArgs]{
		Args: services.SSHPullArgs{HostID: "test-host-id"},
	}

	err = worker.Work(context.Background(), job)
	require.Error(t, err)
	assert.ErrorIs(t, err, sshErr)
	assert.NotContains(t, err.Error(), "cancel")
}
