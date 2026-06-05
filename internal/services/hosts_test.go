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
	"google.golang.org/protobuf/proto"
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

type mockSSHPullRunner struct {
	services.SSHPullRunner
	result           services.SSHPullResult
	err              error
	calledHost       string
	calledPrivateKey string
}

func (m *mockSSHPullRunner) Collect(ctx context.Context, privateKeyPEM string, user string, host string) (services.SSHPullResult, error) {
	m.calledHost = host
	m.calledPrivateKey = privateKeyPEM
	return m.result, m.err
}

func TestHosts_RunSSHPull_ProducesMatcherDecisions(t *testing.T) {
	snapBytes := mockProtobufPayload(t)

	mockRunner := &mockSSHPullRunner{
		result: services.SSHPullResult{
			MachineID:        "machine-ssh-999",
			Hostname:         "ssh-host-test",
			IPAddress:        "10.0.0.99",
			OSFamily:         "rpm",
			OSName:           "Rocky Linux",
			OSVersion:        "9.3",
			OSMajor:          9,
			Architecture:     "x86_64",
			RunningKernel:    "5.14.0",
			CollectedAt:      time.Now().UTC(),
			AvailableUpdates: 1,
			Payload:          snapBytes,
			OverallAction:    "update_package",
		},
	}

	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.SSHPullRunner](i, func(i do.Injector) (services.SSHPullRunner, error) {
				return mockRunner, nil
			})
		}),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	// Seed product stream and advisory rule so matcher can match
	_, err := backend.DB().Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active')
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ('RLSA-2023:9999', 'rocky_errata_api', '9999', 'rocky', 'security', 'critical', 'Vulnerability', 'vendor_db', true)
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ('RLSA-2023:9999', 'rocky:9-baseos')
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, evidence_tier)
		VALUES ('rule_9999', 'RLSA-2023:9999', 'rocky:9-baseos', 'openssl', '< 0:3.0.7-2.el9', 'vendor_db')
	`)
	require.NoError(t, err)

	// Create SSH pull host
	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:          hostID,
		DisplayName: utils.Some("ssh-host-test"),
		Hostname:    utils.Some("ssh-target.example.com"),
	})
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		UPDATE hosts
		SET onboarding_mode = 'ssh', approval_status = 'approved'
		WHERE id = $1
	`, hostID)
	require.NoError(t, err)

	crypto := do.MustInvoke[utils.Crypto](backend.Injector())
	encryptedKey, err := crypto.Encrypt("mock-private-key")
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_ssh_pull (host_id, pull_hostname, pull_ssh_user, pull_frequency_minutes, pull_private_key, onboarded)
		VALUES ($1, 'ssh-target.example.com', 'root', 60, $2, true)
	`, hostID, encryptedKey)
	require.NoError(t, err)

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	err = hostsService.RunSSHPull(ctx, string(hostID))
	require.NoError(t, err)

	// Assert that a snapshot was created and decision records were generated by the matcher!
	snapshot, err := queries.GetLatestHostSnapshotByHostID(ctx, hostID)
	require.NoError(t, err)

	updatedHost, err := queries.GetHostByID(ctx, hostID)
	require.NoError(t, err)
	require.True(t, updatedHost.LastSnapshotID.IsPresent())
	assert.Equal(t, snapshot.ID, updatedHost.LastSnapshotID.UnwrapOr(""))
	assert.Equal(t, "ssh-host-test", updatedHost.Hostname.UnwrapOr(""))
	assert.Equal(t, "ssh-target.example.com:22", mockRunner.calledHost)

	err = hostsService.RunSSHPull(ctx, string(hostID))
	require.NoError(t, err)
	assert.Equal(t, "ssh-target.example.com:22", mockRunner.calledHost)

	decisions, err := queries.ListDecisionPageRowsBySnapshot(ctx, snapshot.ID)
	require.NoError(t, err)
	assert.Len(t, decisions, 1)
	assert.Equal(t, "RLSA-2023:9999", decisions[0].AdvisoryID)
	assert.Equal(t, "openssl", decisions[0].PackageName)
}

func TestHosts_RunSSHPull_CollectErrorPreserved(t *testing.T) {
	mockRunner := &mockSSHPullRunner{err: fmt.Errorf("ssh failed: permission denied")}

	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.SSHPullRunner](i, func(_ do.Injector) (services.SSHPullRunner, error) {
				return mockRunner, nil
			})
		}),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	hostID := id.New("h")
	_, err := queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:          hostID,
		DisplayName: utils.Some("ssh-host-typed-nil"),
		Hostname:    utils.Some("ssh-host-typed-nil"),
	})
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		UPDATE hosts
		SET onboarding_mode = 'ssh', approval_status = 'approved'
		WHERE id = $1
	`, hostID)
	require.NoError(t, err)

	crypto := do.MustInvoke[utils.Crypto](backend.Injector())
	encryptedKey, err := crypto.Encrypt("mock-private-key")
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_ssh_pull (host_id, pull_hostname, pull_ssh_user, pull_frequency_minutes, pull_private_key, onboarded)
		VALUES ($1, 'ssh-host-typed-nil', 'root', 60, $2, true)
	`, hostID, encryptedKey)
	require.NoError(t, err)

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	err = hostsService.RunSSHPull(ctx, string(hostID))
	require.Error(t, err)
	assert.ErrorContains(t, err, "collect snapshot: ssh failed: permission denied")
	assert.NotContains(t, err.Error(), "%!w(<nil>)")

	jobs, err := hostsService.ListSSHPullJobs(ctx, string(hostID))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "failed", jobs[0].Status)
	require.NotNil(t, jobs[0].Error)
	assert.Contains(t, *jobs[0].Error, "ssh failed: permission denied")
}

func mockProtobufPayload(t *testing.T) []byte {
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "ssh-host-test",
			OsName:       "Rocky Linux",
			OsVersion:    "9.3",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "openssl",
				Epoch:   0,
				Version: "3.0.7",
				Release: "1.el9",
				Arch:    "x86_64",
				Nevra:   "openssl-0:3.0.7-1.el9.x86_64",
			},
		},
		Repos: []*agentpb.Repo{
			{
				RepoId:  "baseos",
				Enabled: true,
			},
		},
		Runtime: &agentpb.Runtime{
			KernelRunning: "5.14.0",
		},
	}
	b, err := proto.Marshal(snap)
	require.NoError(t, err)
	return b
}

func TestHosts_IngestManualReport(t *testing.T) {
	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	// Seed product stream and advisory rule so matcher can match
	_, err := backend.DB().Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active')
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ('RLSA-2023:9999', 'rocky_errata_api', '9999', 'rocky', 'security', 'critical', 'Vulnerability', 'vendor_db', true)
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ('RLSA-2023:9999', 'rocky:9-baseos')
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, evidence_tier)
		VALUES ('rule_9999', 'RLSA-2023:9999', 'rocky:9-baseos', 'openssl', '< 0:3.0.7-2.el9', 'vendor_db')
	`)
	require.NoError(t, err)

	hostsService := do.MustInvoke[services.Hosts](backend.Injector())

	// Create Manual Host
	hostInfo, err := hostsService.CreateManualHost(ctx, "manual-test", "manual-host-test")
	require.NoError(t, err)
	assert.Equal(t, "manual", hostInfo.OnboardingMode)
	assert.Equal(t, "approved", hostInfo.ApprovalStatus)

	reportContent := `_PB_METADATA_HOSTNAME=manual-host-test
_PB_METADATA_ARCH=x86_64
_PB_METADATA_KERNEL=5.14.0
_PB_METADATA_MACHINE_ID=machine-manual-999
_PB_METADATA_IP=10.0.0.99
_PB_METADATA_BOOT_TIME=1672531199
_PB_METADATA_OS_ID=rocky
_PB_METADATA_OS_NAME=Rocky Linux
_PB_METADATA_OS_VERSION=9.3
---UPDATES_START---
openssl.x86_64 3.0.7-2.el9 baseos
---PACKAGES_START---
openssl|0|3.0.7|1.el9|x86_64|openssl-3.0.7-1.el9.src.rpm|Rocky
---REPOS_START---
repo-baseos Enabled`

	err = hostsService.IngestManualReport(ctx, hostInfo.ID, []byte(reportContent))
	require.NoError(t, err)

	// Verify snapshot and matching decision
	snapshot, err := queries.GetLatestHostSnapshotByHostID(ctx, hostInfo.ID)
	require.NoError(t, err)

	updatedHost, err := queries.GetHostByID(ctx, hostInfo.ID)
	require.NoError(t, err)
	require.True(t, updatedHost.LastSnapshotID.IsPresent())
	assert.Equal(t, snapshot.ID, updatedHost.LastSnapshotID.UnwrapOr(""))
	assert.Equal(t, "manual-host-test", updatedHost.Hostname.UnwrapOr(""))
	assert.Equal(t, "Rocky Linux", updatedHost.OsName)
	assert.Equal(t, "9.3", updatedHost.OsVersion)

	decisions, err := queries.ListDecisionPageRowsBySnapshot(ctx, snapshot.ID)
	require.NoError(t, err)
	assert.Len(t, decisions, 1)
	assert.Equal(t, "RLSA-2023:9999", decisions[0].AdvisoryID)
	assert.Equal(t, "openssl", decisions[0].PackageName)
}

func TestHosts_SSHUniqueKeyPairFlagAndGlobalFallback(t *testing.T) {
	mockRunner := &mockSSHPullRunner{
		result: services.SSHPullResult{
			MachineID:        "machine-ssh-999",
			Hostname:         "ssh-host-test",
			IPAddress:        "10.0.0.12",
			OSFamily:         "rpm",
			OSName:           "Rocky Linux",
			OSVersion:        "9.3",
			OSMajor:          9,
			Architecture:     "x86_64",
			RunningKernel:    "5.14.0",
			CollectedAt:      time.Now().UTC(),
			AvailableUpdates: 0,
			Payload:          mockProtobufPayload(t),
			OverallAction:    "none",
		},
	}

	backend := apitesting.NewBackend(t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.SSHPullRunner](i, func(i do.Injector) (services.SSHPullRunner, error) {
				return mockRunner, nil
			})
		}),
	)

	queries := do.MustInvoke[db.Querier](backend.Injector())
	hostsService := do.MustInvoke[services.Hosts](backend.Injector())
	settingsService := do.MustInvoke[services.Settings](backend.Injector())
	ctx := context.Background()

	// 1. Create a host using global SSH key (UniqueKeyPair = false)
	resGlobal, err := hostsService.CreateSSHHost(ctx, services.CreateSSHHostInput{
		DisplayName:      "global-key-host",
		Hostname:         "global.example.com",
		SSHUser:          "root",
		FrequencyMinutes: 60,
		UniqueKeyPair:    false,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resGlobal.HostID)

	// Fetch global SSH key to make sure it matches
	globalKey, err := settingsService.GetGlobalSSHKeyPair(ctx)
	require.NoError(t, err)
	assert.Equal(t, globalKey.PublicKey, resGlobal.PublicKey)

	// Verify columns in DB are NULL/empty
	cfgGlobal, err := queries.GetSSHPullConfigByHostID(ctx, resGlobal.HostID)
	require.NoError(t, err)
	assert.Empty(t, cfgGlobal.PullPublicKey.UnwrapOr(""))
	assert.Empty(t, cfgGlobal.PullPrivateKey.UnwrapOr(""))

	// Onboard and run SSH pull
	err = hostsService.OnboardSSHHost(ctx, resGlobal.HostID)
	require.NoError(t, err)

	err = hostsService.RunSSHPull(ctx, resGlobal.HostID)
	require.NoError(t, err)
	assert.Equal(t, globalKey.PrivateKey, mockRunner.calledPrivateKey)

	// 2. Create a host using unique SSH key (UniqueKeyPair = true)
	resUnique, err := hostsService.CreateSSHHost(ctx, services.CreateSSHHostInput{
		DisplayName:      "unique-key-host",
		Hostname:         "unique.example.com",
		SSHUser:          "root",
		FrequencyMinutes: 60,
		UniqueKeyPair:    true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resUnique.HostID)
	assert.NotEqual(t, globalKey.PublicKey, resUnique.PublicKey)

	// Verify columns in DB are NOT NULL/empty
	cfgUnique, err := queries.GetSSHPullConfigByHostID(ctx, resUnique.HostID)
	require.NoError(t, err)
	assert.NotEmpty(t, cfgUnique.PullPublicKey.UnwrapOr(""))
	assert.NotEmpty(t, cfgUnique.PullPrivateKey.UnwrapOr(""))

	// Onboard and run SSH pull
	err = hostsService.OnboardSSHHost(ctx, resUnique.HostID)
	require.NoError(t, err)

	err = hostsService.RunSSHPull(ctx, resUnique.HostID)
	require.NoError(t, err)
	assert.NotEqual(t, globalKey.PrivateKey, mockRunner.calledPrivateKey)
	assert.NotEmpty(t, mockRunner.calledPrivateKey)
}
