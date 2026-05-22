package services_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/services"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
)

type failingAdvisorySyncService struct{}

func (f *failingAdvisorySyncService) SyncScope(ctx context.Context, scopeKey string) error {
	return fmt.Errorf("mock error")
}

func (f *failingAdvisorySyncService) GetScopeStatuses(ctx context.Context) ([]services.AdvisoryScopeStatus, error) {
	return nil, fmt.Errorf("mock error")
}

func (f *failingAdvisorySyncService) TriggerManualSync(ctx context.Context, scopeKey string) error {
	return fmt.Errorf("mock error")
}

func (f *failingAdvisorySyncService) GetOverview(ctx context.Context) (services.AdvisoryOverview, error) {
	return services.AdvisoryOverview{}, fmt.Errorf("mock error")
}

func (f *failingAdvisorySyncService) ResolveScopeKey(ctx context.Context, osFamily, osName, osVersion string, osMajor int32, arch string) (string, error) {
	return "", fmt.Errorf("mock error")
}

func (f *failingAdvisorySyncService) RegisterScopeDemand(ctx context.Context, scopeKey string) error {
	return fmt.Errorf("mock error")
}

var _ services.AdvisorySyncService = (*failingAdvisorySyncService)(nil)

func TestHostIngestion_ResilientToAdvisorySyncFailure(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.AdvisorySyncService](i, func(i do.Injector) (services.AdvisorySyncService, error) {
				return &failingAdvisorySyncService{}, nil
			})
		}),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	// 1. Create and approve a host in the database
	host, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           id.New("h"),
		DisplayName:  utils.Some("test-host"),
		MachineID:    utils.Some("machine-123"),
		Hostname:     utils.Some("test-host"),
		IpAddress:    utils.Some("10.0.0.1"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, host.ID)
	require.NoError(t, err)

	// Insert host access token
	token := "pb_host_testtoken1234567890123"
	_, err = queries.InsertHostAccessToken(ctx, db.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    host.ID,
		TokenHash: utils.SHA256(token),
	})
	require.NoError(t, err)

	// 2. Ingest agent snapshot with the mocked failing advisory service
	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "machine-123",
			Hostname:                    "test-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_APT,
			OsName:                      "Ubuntu",
			OsMajor:                     22,
			OsVersion:                   "22.04",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 2,
			IpAddresses:                 []string{"10.0.0.1"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-foo"},
	}

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	syncResp, err := hostsService.IngestAgentSnapshot(ctx, token, snapshot)

	// 3. Ingestion should succeed and return sync response without error
	require.NoError(t, err)
	assert.NotNil(t, syncResp)
	assert.NotEmpty(t, syncResp.SnapshotId)
}

type partialFailingAdvisorySyncService struct {
	services.AdvisorySyncService
}

func (f *partialFailingAdvisorySyncService) ResolveScopeKey(ctx context.Context, osFamily, osName, osVersion string, osMajor int32, arch string) (string, error) {
	return "non-existent-scope", nil
}

func (f *partialFailingAdvisorySyncService) RegisterScopeDemand(ctx context.Context, scopeKey string) error {
	return nil
}

func TestHostIngestion_ResilientToUpdateHostAdvisoryScopeKeyFailure(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.AdvisorySyncService](i, func(i do.Injector) (services.AdvisorySyncService, error) {
				return &partialFailingAdvisorySyncService{}, nil
			})
		}),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	// 1. Create and approve a host in the database
	host, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           id.New("h"),
		DisplayName:  utils.Some("test-host"),
		MachineID:    utils.Some("machine-123"),
		Hostname:     utils.Some("test-host"),
		IpAddress:    utils.Some("10.0.0.1"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, host.ID)
	require.NoError(t, err)

	// Insert host access token
	token := "pb_host_testtoken1234567890123"
	_, err = queries.InsertHostAccessToken(ctx, db.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    host.ID,
		TokenHash: utils.SHA256(token),
	})
	require.NoError(t, err)

	// 2. Ingest agent snapshot. ResolveScopeKey returns "non-existent-scope",
	// so the UpdateHostAdvisoryScopeKey query will violate the foreign key constraint.
	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "machine-123",
			Hostname:                    "test-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_APT,
			OsName:                      "Ubuntu",
			OsMajor:                     22,
			OsVersion:                   "22.04",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 2,
			IpAddresses:                 []string{"10.0.0.1"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-foo"},
	}

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	syncResp, err := hostsService.IngestAgentSnapshot(ctx, token, snapshot)

	// 3. Ingestion should succeed and return sync response without error, despite the FK failure.
	require.NoError(t, err)
	assert.NotNil(t, syncResp)
	assert.NotEmpty(t, syncResp.SnapshotId)

	// 4. Verify that the snapshot was actually saved in the DB (confirming transaction committed).
	dbSnapshot, err := queries.GetLatestHostSnapshotByHostID(ctx, host.ID)
	require.NoError(t, err)
	assert.Equal(t, host.ID, dbSnapshot.HostID)
}

func TestHostIngestion_RepeatedSnapshotsKeepAdvisorySyncRateBounded(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	pool := do.MustInvoke[*pgxpool.Pool](backend.Injector())
	ctx := context.Background()

	// Seed advisory scope to satisfy foreign key constraint on hosts table
	_, err := pool.Exec(ctx, "INSERT INTO advisory_scopes (scope_key) VALUES ('ubuntu:jammy')")
	require.NoError(t, err)

	riverClient := do.MustInvoke[*river.Client[pgx.Tx]](backend.Injector())
	err = riverClient.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = riverClient.Stop(ctx)
	})

	// 1. Create and approve a host in the database
	host, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           id.New("h"),
		DisplayName:  utils.Some("test-host"),
		MachineID:    utils.Some("machine-123"),
		Hostname:     utils.Some("test-host"),
		IpAddress:    utils.Some("10.0.0.1"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, host.ID)
	require.NoError(t, err)

	// Insert host access token
	token := "pb_host_testtoken1234567890123"
	_, err = queries.InsertHostAccessToken(ctx, db.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    host.ID,
		TokenHash: utils.SHA256(token),
	})
	require.NoError(t, err)

	// Clear any pre-existing advisory_sync jobs
	_, err = pool.Exec(ctx, "DELETE FROM river_job WHERE kind = 'advisory_sync'")
	require.NoError(t, err)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "machine-123",
			Hostname:                    "test-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_APT,
			OsName:                      "Ubuntu",
			OsMajor:                     22,
			OsVersion:                   "22.04",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 2,
			IpAddresses:                 []string{"10.0.0.1"},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-foo"},
	}

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())

	// Call IngestAgentSnapshot multiple times
	for i := 0; i < 3; i++ {
		_, err := hostsService.IngestAgentSnapshot(ctx, token, snapshot)
		require.NoError(t, err)
	}

	// Verify that the count of advisory_sync jobs is at most 1
	var count int
	require.Eventually(t, func() bool {
		err = pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'advisory_sync'").Scan(&count)
		return err == nil && count == 1
	}, 2*time.Second, 50*time.Millisecond)
}

type hostsMockPeriodicJobManager struct {
	services.PeriodicJobManager
	removeErr     error
	removedHostID string
}

func (m *hostsMockPeriodicJobManager) RemoveSSHPullJob(ctx context.Context, hostID string) error {
	m.removedHostID = hostID
	return m.removeErr
}

func TestHosts_DeleteHost_HandlesPeriodicJobRemovalError(t *testing.T) {
	mockMgr := &hostsMockPeriodicJobManager{
		removeErr: fmt.Errorf("mock remove error"),
	}
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.PeriodicJobManager](i, func(i do.Injector) (services.PeriodicJobManager, error) {
				return mockMgr, nil
			})
		}),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	// Create and approve a host in the database
	host, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           id.New("h"),
		DisplayName:  utils.Some("test-host"),
		MachineID:    utils.Some("machine-123"),
		Hostname:     utils.Some("test-host"),
		IpAddress:    utils.Some("10.0.0.1"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, host.ID)
	require.NoError(t, err)

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	err = hostsService.DeleteHost(ctx, string(host.ID))
	require.NoError(t, err)
	assert.Equal(t, string(host.ID), mockMgr.removedHostID)

	// Assert that the host is successfully deleted from the database.
	_, err = queries.GetHostByID(ctx, host.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}
