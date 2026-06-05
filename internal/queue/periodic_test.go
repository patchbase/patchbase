package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/queue"
	apitesting "go.patchbase.net/server/internal/testing"
)

func TestPeriodicJobManager_Initialize(t *testing.T) {
	ctx := context.Background()

	backend := apitesting.NewBackend(t)
	pool := do.MustInvoke[*pgxpool.Pool](backend.Injector())

	// 1. Seed advisory scopes
	_, err := pool.Exec(ctx, "INSERT INTO advisory_scopes (scope_key) VALUES ('test:scope-1'), ('test:scope-2')")
	require.NoError(t, err)

	// 2. Seed hosts and host_ssh_pull (approved and onboarded, or approved but not onboarded)
	_, err = pool.Exec(ctx, `
		INSERT INTO hosts (id, onboarding_mode, approval_status)
		VALUES ('h_host1', 'ssh', 'approved'), ('h_host2', 'ssh', 'approved'), ('h_host3', 'ssh', 'approved')
	`)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO host_ssh_pull (host_id, pull_hostname, pull_frequency_minutes, onboarded)
		VALUES ('h_host1', 'h_host1', 60, true), ('h_host2', 'h_host2', 120, true), ('h_host3', 'h_host3', 180, false)
	`)
	require.NoError(t, err)

	// 3. Create PeriodicJobManager
	manager, err := queue.NewPeriodicJobManager(backend.Injector())
	require.NoError(t, err)

	concreteManager, ok := manager.(*queue.PeriodicJobManager)
	require.True(t, ok)

	// Verify maps are empty before initialization
	assert.Equal(t, 0, concreteManager.GetSyncJobsCountForTest())
	assert.Equal(t, 0, concreteManager.GetSSHJobsCountForTest())

	// 4. Initialize
	err = concreteManager.Initialize(ctx)
	require.NoError(t, err)

	// Verify jobs are registered in internal maps
	assert.Equal(t, 2, concreteManager.GetSyncJobsCountForTest())
	assert.True(t, concreteManager.HasSyncJobForTest("test:scope-1"))
	assert.True(t, concreteManager.HasSyncJobForTest("test:scope-2"))

	assert.Equal(t, 2, concreteManager.GetSSHJobsCountForTest())
	assert.True(t, concreteManager.HasSSHJobForTest("h_host1"))
	assert.True(t, concreteManager.HasSSHJobForTest("h_host2"))
	assert.False(t, concreteManager.HasSSHJobForTest("h_host3")) // Should not be registered because onboarded = false
}

func TestPeriodicJobManager_AddAndRemove(t *testing.T) {
	ctx := context.Background()

	backend := apitesting.NewBackend(t)

	manager, err := queue.NewPeriodicJobManager(backend.Injector())
	require.NoError(t, err)

	concreteManager, ok := manager.(*queue.PeriodicJobManager)
	require.True(t, ok)

	// Test dynamic addition of Advisory Sync job
	err = concreteManager.AddAdvisorySyncJob(ctx, "dynamic-scope", true)
	require.NoError(t, err)
	assert.True(t, concreteManager.HasSyncJobForTest("dynamic-scope"))

	// Test dynamic removal of Advisory Sync job
	err = concreteManager.RemoveAdvisorySyncJob(ctx, "dynamic-scope")
	require.NoError(t, err)
	assert.False(t, concreteManager.HasSyncJobForTest("dynamic-scope"))

	// Test dynamic addition of SSH Pull job
	err = concreteManager.AddSSHPullJob(ctx, "h_dynamic_host", 30, true)
	require.NoError(t, err)
	assert.True(t, concreteManager.HasSSHJobForTest("h_dynamic_host"))

	// Test dynamic removal of SSH Pull job
	err = concreteManager.RemoveSSHPullJob(ctx, "h_dynamic_host")
	require.NoError(t, err)
	assert.False(t, concreteManager.HasSSHJobForTest("h_dynamic_host"))
}

func TestPeriodicJobManager_DuplicateRegistration(t *testing.T) {
	ctx := context.Background()

	backend := apitesting.NewBackend(t)
	pool := do.MustInvoke[*pgxpool.Pool](backend.Injector())
	riverClient := do.MustInvoke[*river.Client[pgx.Tx]](backend.Injector())

	err := riverClient.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = riverClient.Stop(ctx)
	})

	manager, err := queue.NewPeriodicJobManager(backend.Injector())
	require.NoError(t, err)

	concreteManager, ok := manager.(*queue.PeriodicJobManager)
	require.True(t, ok)

	// Clear any pre-existing advisory_sync jobs
	_, err = pool.Exec(ctx, "DELETE FROM river_job WHERE kind = 'advisory_sync'")
	require.NoError(t, err)

	// Seed advisory scope to satisfy the SyncScope database updates
	_, err = pool.Exec(ctx, "INSERT INTO advisory_scopes (scope_key) VALUES ('duplicate-scope')")
	require.NoError(t, err)

	// 1. Add it the first time
	err = concreteManager.AddAdvisorySyncJob(ctx, "duplicate-scope", true)
	require.NoError(t, err)
	assert.True(t, concreteManager.HasSyncJobForTest("duplicate-scope"))

	var countFirst int
	require.Eventually(t, func() bool {
		err = pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'advisory_sync'").Scan(&countFirst)
		return err == nil && countFirst == 1
	}, 2*time.Second, 50*time.Millisecond)

	// 2. Add it again (should not error and should not add duplicate to River queue)
	err = concreteManager.AddAdvisorySyncJob(ctx, "duplicate-scope", true)
	require.NoError(t, err)
	assert.True(t, concreteManager.HasSyncJobForTest("duplicate-scope"))

	// Wait a bit to ensure no new job is inserted
	time.Sleep(100 * time.Millisecond)

	var countSecond int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'advisory_sync'").Scan(&countSecond)
	require.NoError(t, err)
	assert.Equal(t, 1, countSecond)
}
