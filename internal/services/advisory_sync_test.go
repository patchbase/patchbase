package services_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/services"
	db "go.patchbase.net/server/internal/sql"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"

	_ "modernc.org/sqlite"
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
			name:      "Ubuntu Resolute",
			osFamily:  "debian",
			osName:    "Ubuntu",
			osVersion: "26.04",
			osMajor:   26,
			arch:      "x86_64",
			expected:  "ubuntu:resolute",
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

func (m *mockPeriodicJobManager) AddAdvisorySyncJob(ctx context.Context, scopeKey string, runOnStart bool) error {
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

func createTestSQLiteDBForSync(t *testing.T) (*sql.DB, string) {
	tmpFile, err := os.CreateTemp("", "patchbase-test-sqlite-*.db")
	require.NoError(t, err)
	dbPath := tmpFile.Name()
	_ = tmpFile.Close()

	sqliteDB, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	_, err = sqliteDB.Exec(`
		CREATE TABLE product_streams (
			id TEXT PRIMARY KEY,
			vendor TEXT,
			distro_family TEXT,
			distro_name TEXT,
			major_version INTEGER,
			minor_version TEXT,
			architecture TEXT,
			repo_family TEXT,
			repo_id_pattern TEXT,
			cpe TEXT,
			status TEXT
		);
		CREATE TABLE advisories (
			id TEXT PRIMARY KEY,
			source_system TEXT,
			raw_source_id TEXT,
			source_url TEXT,
			vendor TEXT,
			advisory_type TEXT,
			severity TEXT,
			summary TEXT,
			description TEXT,
			published_at TEXT,
			updated_at TEXT,
			evidence_tier TEXT,
			is_security INTEGER
		);
		CREATE TABLE advisory_references (
			id TEXT PRIMARY KEY,
			advisory_id TEXT,
			ref_type TEXT,
			ref_value TEXT,
			severity_vendor TEXT,
			severity_cvss REAL,
			title TEXT,
			url TEXT
		);
		CREATE TABLE advisory_product_streams (
			advisory_id TEXT,
			product_stream_id TEXT,
			PRIMARY KEY (advisory_id, product_stream_id)
		);
		CREATE TABLE affected_package_rules (
			id TEXT PRIMARY KEY,
			advisory_id TEXT,
			product_stream_id TEXT,
			package_name TEXT,
			source_rpm TEXT,
			arch TEXT,
			epoch_constraint TEXT,
			version_constraint TEXT,
			release_constraint TEXT,
			rpm_evr_rule TEXT,
			context TEXT,
			evidence_tier TEXT
		);
		CREATE TABLE fixed_packages (
			id TEXT PRIMARY KEY,
			advisory_id TEXT,
			product_stream_id TEXT,
			package_name TEXT,
			epoch INTEGER,
			version TEXT,
			release TEXT,
			arch TEXT,
			nevra TEXT,
			source_rpm TEXT,
			repo_family TEXT,
			evidence_tier TEXT
		);
	`)
	require.NoError(t, err)

	return sqliteDB, dbPath
}

func TestAdvisorySync_SyncScope_Caching(t *testing.T) {
	// 1. Create a populated test SQLite DB
	sqliteDB, dbPath := createTestSQLiteDBForSync(t)
	defer func() { _ = os.Remove(dbPath) }()
	defer func() { _ = sqliteDB.Close() }()

	_, err := sqliteDB.Exec(`
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active');

		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ('RLSA-2023:9999', 'rocky_errata_api', '9999', 'rocky', 'security', 'critical', 'Summary', 'vendor_db', 1);
	`)
	require.NoError(t, err)

	// Compute hash of the sqlite file
	dbBytes, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	hasher := sha256.New()
	hasher.Write(dbBytes)
	dbHash := hex.EncodeToString(hasher.Sum(nil))

	// 2. Set up mock HTTP server
	var manifestRequests int
	var dbRequests int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest.json" {
			manifestRequests++
			manifest := services.Manifest{
				SchemaVersion: 1,
				GeneratedAt:   time.Now().Format(time.RFC3339),
				Scopes: []services.ScopeDetail{
					{
						Key:           "rocky:9",
						Path:          "advisories-rocky-9.db",
						Sha256:        dbHash,
						SizeBytes:     int64(len(dbBytes)),
						AdvisoryCount: 1,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifest)
			return
		}

		if r.URL.Path == "/advisories-rocky-9.db" {
			dbRequests++
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(dbBytes)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// 3. Create temp storage directory
	tempStorageDir, err := os.MkdirTemp("", "patchbase-storage-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempStorageDir) }()

	// Create backend override
	backend := apitesting.NewBackend(t)

	// Override config inside the backend's injector
	origCfg := backend.Config()
	customCfg := origCfg
	customCfg.AdvisorySync.BaseURL = srv.URL
	customCfg.AdvisorySync.StorageDir = tempStorageDir

	do.OverrideValue[config.Config](backend.Injector(), customCfg)

	svc := do.MustInvoke[services.AdvisorySyncService](backend.Injector())
	ctx := context.Background()

	// Register demand to initialize scope row
	err = svc.RegisterScopeDemand(ctx, "rocky:9")
	require.NoError(t, err)

	// --- FIRST SYNC (no cache) ---
	err = svc.SyncScope(ctx, "rocky:9")
	require.NoError(t, err)

	assert.Equal(t, 1, manifestRequests, "should have requested manifest once")
	assert.Equal(t, 1, dbRequests, "should have requested db once")

	queries := do.MustInvoke[db.Querier](backend.Injector())
	scope, err := queries.GetAdvisoryScope(ctx, "rocky:9")
	require.NoError(t, err)
	assert.Equal(t, "synced", scope.Status)
	assert.Equal(t, dbHash, scope.Sha256.UnwrapOr(""))

	// Reset request counts
	manifestRequests = 0
	dbRequests = 0

	// --- SECOND SYNC (cached) ---
	err = svc.SyncScope(ctx, "rocky:9")
	require.NoError(t, err)

	assert.Equal(t, 1, manifestRequests, "should request manifest to check for updates")
	assert.Equal(t, 0, dbRequests, "should NOT request database because hash hasn't changed")

	// Ensure it's still synced
	scope, err = queries.GetAdvisoryScope(ctx, "rocky:9")
	require.NoError(t, err)
	assert.Equal(t, "synced", scope.Status)

	// --- THIRD SYNC (retry path; status not synced, same hash already imported) ---
	_, err = queries.UpdateAdvisoryScopeStatus(ctx, db.UpdateAdvisoryScopeStatusParams{
		ScopeKey:  "rocky:9",
		Status:    "failed",
		LastError: utils.Some("transient failure"),
	})
	require.NoError(t, err)

	manifestRequests = 0
	dbRequests = 0

	err = svc.SyncScope(ctx, "rocky:9")
	require.NoError(t, err)
	assert.Equal(t, 1, manifestRequests, "should still request manifest")
	assert.Equal(t, 0, dbRequests, "should NOT request database on retry when hash is unchanged and previously imported")

	scope, err = queries.GetAdvisoryScope(ctx, "rocky:9")
	require.NoError(t, err)
	assert.Equal(t, "synced", scope.Status)
}
