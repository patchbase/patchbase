package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.patchbase.net/server/internal/services"
	db "go.patchbase.net/server/internal/sql"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
)

func TestAdvisorySync_ResolveScopeKey(t *testing.T) {
	backend := apitesting.NewBackend(t)
	svc := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	ctx := context.Background()

	tests := []struct {
		name      string
		osFamily  string
		osName    string
		osVersion string
		osMajor   int32
		arch      string
		expected  string
	}{
		{
			name:      "Ubuntu Noble",
			osFamily:  "debian",
			osName:    "Ubuntu",
			osVersion: "24.04",
			osMajor:   24,
			arch:      "x86_64",
			expected:  "ubuntu:noble",
		},
		{
			name:      "Ubuntu Jammy",
			osFamily:  "debian",
			osName:    "Ubuntu",
			osVersion: "22.04",
			osMajor:   22,
			arch:      "x86_64",
			expected:  "ubuntu:jammy",
		},
		{
			name:      "Debian Bookworm DSA",
			osFamily:  "debian",
			osName:    "Debian GNU/Linux",
			osVersion: "12",
			osMajor:   12,
			arch:      "x86_64",
			expected:  "debian:bookworm-dsa",
		},
		{
			name:      "Rocky Linux 9",
			osFamily:  "rhel",
			osName:    "Rocky Linux",
			osVersion: "9.3",
			osMajor:   9,
			arch:      "x86_64",
			expected:  "rocky:9",
		},
		{
			name:      "AlmaLinux 9",
			osFamily:  "rhel",
			osName:    "AlmaLinux",
			osVersion: "9.2",
			osMajor:   9,
			arch:      "x86_64",
			expected:  "alma:9",
		},
		{
			name:      "AlmaLinux 10",
			osFamily:  "rhel",
			osName:    "AlmaLinux",
			osVersion: "10.0",
			osMajor:   10,
			arch:      "x86_64",
			expected:  "alma:10",
		},
		{
			name:      "RHEL 9",
			osFamily:  "rhel",
			osName:    "Red Hat Enterprise Linux",
			osVersion: "9.4",
			osMajor:   9,
			arch:      "x86_64",
			expected:  "rhel:9",
		},
		{
			name:      "RHEL 10",
			osFamily:  "rhel",
			osName:    "Red Hat Enterprise Linux",
			osVersion: "10.0",
			osMajor:   10,
			arch:      "x86_64",
			expected:  "rhel:10",
		},
		{
			name:      "Unknown OS",
			osFamily:  "unknown",
			osName:    "Unknown OS",
			osVersion: "1.0",
			osMajor:   1,
			arch:      "x86_64",
			expected:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ResolveScopeKey(ctx, tc.osFamily, tc.osName, tc.osVersion, tc.osMajor, tc.arch)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestAdvisorySync_RegisterScopeDemand_NoClobber(t *testing.T) {
	backend := apitesting.NewBackend(t)
	svc := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	scopeKey := "ubuntu:noble"

	// 1. Manually insert fully synced scope metadata
	now := time.Now().UTC()
	_, err := queries.UpsertAdvisoryScope(ctx, db.UpsertAdvisoryScopeParams{
		ScopeKey:      scopeKey,
		Status:        "synced",
		LastSyncAt:    pgtype.Timestamptz{Time: now, Valid: true},
		LastSuccessAt: pgtype.Timestamptz{Time: now, Valid: true},
		LastError:     utils.None[string](),
		AdvisoryCount: 42,
		Sha256:        utils.Some("dummy-sha256"),
		SizeBytes:     1024,
		LocalPath:     utils.Some("/path/to/db"),
		NextRefreshAt: pgtype.Timestamptz{Time: now.Add(6 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	// 2. Register demand on the same scope
	err = svc.RegisterScopeDemand(ctx, scopeKey)
	require.NoError(t, err)

	// 3. Fetch scope from DB and assert synced metadata is NOT clobbered
	res, err := queries.GetAdvisoryScope(ctx, scopeKey)
	require.NoError(t, err)

	assert.Equal(t, "synced", res.Status)
	assert.Equal(t, int32(42), res.AdvisoryCount)
	assert.Equal(t, "dummy-sha256", res.Sha256.UnwrapOr(""))
	assert.Equal(t, int64(1024), res.SizeBytes)
	assert.Equal(t, "/path/to/db", res.LocalPath.UnwrapOr(""))
}

func TestAdvisorySync_TriggerManualSync_UnknownScope(t *testing.T) {
	backend := apitesting.NewBackend(t)
	svc := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	scopeKey := "debian:bookworm-dsa"

	// 1. Verify scope does not exist yet
	_, err := queries.GetAdvisoryScope(ctx, scopeKey)
	require.Error(t, err)

	// 2. Trigger manual sync for this unknown scope
	err = svc.TriggerManualSync(ctx, scopeKey)
	require.NoError(t, err)

	// 3. Verify scope was created in pending status
	res, err := queries.GetAdvisoryScope(ctx, scopeKey)
	require.NoError(t, err)

	assert.Equal(t, "pending", res.Status)
	assert.Equal(t, scopeKey, res.ScopeKey)
}

func TestAdvisorySync_TriggerManualSync_PreservesMetadata(t *testing.T) {
	backend := apitesting.NewBackend(t)
	svc := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	queries := do.MustInvoke[db.Querier](backend.Injector())
	ctx := context.Background()

	scopeKey := "ubuntu:noble"

	// 1. Manually insert fully synced scope metadata
	now := time.Now().UTC()
	_, err := queries.UpsertAdvisoryScope(ctx, db.UpsertAdvisoryScopeParams{
		ScopeKey:      scopeKey,
		Status:        "synced",
		LastSyncAt:    pgtype.Timestamptz{Time: now, Valid: true},
		LastSuccessAt: pgtype.Timestamptz{Time: now, Valid: true},
		LastError:     utils.None[string](),
		AdvisoryCount: 42,
		Sha256:        utils.Some("dummy-sha256"),
		SizeBytes:     1024,
		LocalPath:     utils.Some("/path/to/db"),
		NextRefreshAt: pgtype.Timestamptz{Time: now.Add(6 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	// 2. Trigger manual sync on the existing scope
	err = svc.TriggerManualSync(ctx, scopeKey)
	require.NoError(t, err)

	// 3. Fetch scope from DB and assert synced metadata is NOT clobbered/reset
	res, err := queries.GetAdvisoryScope(ctx, scopeKey)
	require.NoError(t, err)

	assert.Equal(t, "pending", res.Status) // Manual sync changes status to pending
	assert.Equal(t, int32(42), res.AdvisoryCount)
	assert.Equal(t, "dummy-sha256", res.Sha256.UnwrapOr(""))
	assert.Equal(t, int64(1024), res.SizeBytes)
	assert.Equal(t, "/path/to/db", res.LocalPath.UnwrapOr(""))
}

type mockPeriodicJobManager struct {
	services.PeriodicJobManager
	registeredScopes []string
}

func (m *mockPeriodicJobManager) AddAdvisorySyncJob(ctx context.Context, scopeKey string) error {
	m.registeredScopes = append(m.registeredScopes, scopeKey)
	return nil
}

func TestAdvisorySync_TriggerManualSync_RegistersPeriodicJob(t *testing.T) {
	mockMgr := &mockPeriodicJobManager{}
	backend := apitesting.NewBackend(t,
		apitesting.WithInjectorOverride(func(i do.Injector) {
			do.Override[services.PeriodicJobManager](i, func(i do.Injector) (services.PeriodicJobManager, error) {
				return mockMgr, nil
			})
		}),
	)
	svc := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	ctx := context.Background()

	scopeKey := "debian:bookworm-dsa"

	err := svc.TriggerManualSync(ctx, scopeKey)
	require.NoError(t, err)

	assert.Equal(t, []string{scopeKey}, mockMgr.registeredScopes)
}
