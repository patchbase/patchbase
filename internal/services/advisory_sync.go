package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/samber/do/v2"
	"github.com/spf13/afero"
	"go.patchbase.net/server/internal/config"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"

	// Import pure-Go sqlite driver
	_ "modernc.org/sqlite"
)

type AdvisorySyncArgs struct {
	ScopeKey string `json:"scope_key"`
}

func (AdvisorySyncArgs) Kind() string {
	return "advisory_sync"
}

type AdvisorySyncService interface {
	SyncScope(ctx context.Context, scopeKey string) error
	GetScopeStatuses(ctx context.Context) ([]AdvisoryScopeStatus, error)
	TriggerManualSync(ctx context.Context, scopeKey string) error
	GetOverview(ctx context.Context) (AdvisoryOverview, error)
	ResolveScopeKey(ctx context.Context, osFamily, osName, osVersion string, osMajor int32, arch string) (string, error)
	RegisterScopeDemand(ctx context.Context, scopeKey string) error
}

type AdvisoryScopeStatus struct {
	ScopeKey       string     `json:"scope_key"`
	Status         string     `json:"status"`
	LastSyncAt     *time.Time `json:"last_sync_at"`
	LastSuccessAt  *time.Time `json:"last_success_at"`
	LastError      *string    `json:"last_error"`
	AdvisoryCount  int32      `json:"advisory_count"`
	Sha256         *string    `json:"sha256"`
	SizeBytes      int64      `json:"size_bytes"`
	LocalPath      *string    `json:"local_path"`
	NextRefreshAt  *time.Time `json:"next_refresh_at"`
	HostUsageCount int32      `json:"host_usage_count"`
}

type AdvisoryOverview struct {
	TotalAdvisories int64 `json:"total_advisories"`
	TotalScopes     int32 `json:"total_scopes"`
	SyncedScopes    int32 `json:"synced_scopes"`
}

type advisorySyncService struct {
	config             config.Config
	pool               *pgxpool.Pool
	queries            db.Querier
	fs                 afero.Fs
	httpClient         *http.Client
	injector           do.Injector
	periodicJobManager PeriodicJobManager
}

func NewAdvisorySync(i do.Injector) (AdvisorySyncService, error) {
	cfg, err := do.Invoke[config.Config](i)
	if err != nil {
		return nil, err
	}
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, err
	}
	queries, err := do.Invoke[db.Querier](i)
	if err != nil {
		return nil, err
	}
	periodicJobManager, err := do.Invoke[PeriodicJobManager](i)
	if err != nil {
		return nil, err
	}

	return &advisorySyncService{
		config:             cfg,
		pool:               pool,
		queries:            queries,
		fs:                 afero.NewOsFs(),
		httpClient:         &http.Client{Timeout: 2 * time.Minute},
		injector:           i,
		periodicJobManager: periodicJobManager,
	}, nil
}

type Manifest struct {
	SchemaVersion int           `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Scopes        []ScopeDetail `json:"scopes"`
}

type ScopeDetail struct {
	Key            string `json:"key"`
	Path           string `json:"path"`
	URL            string `json:"url"`
	Sha256         string `json:"sha256"`
	SizeBytes      int64  `json:"size_bytes"`
	UpdatedAt      string `json:"updated_at"`
	AdvisoryCount  int32  `json:"advisory_count"`
	LicenseFeature string `json:"license_feature"`
}

var defaultScopeMappings = []config.ScopeMapping{
	{Match: config.MatchRules{OSName: "Ubuntu", OSVersion: "22.04"}, Scope: "ubuntu:jammy"},
	{Match: config.MatchRules{OSName: "Ubuntu", OSVersion: "24.04"}, Scope: "ubuntu:noble"},
	{Match: config.MatchRules{OSName: "Debian GNU/Linux", OSMajor: 12}, Scope: "debian:bookworm-dsa"},
	{Match: config.MatchRules{OSName: "Debian GNU/Linux", OSMajor: 13}, Scope: "debian:trixie-dsa"},
	{Match: config.MatchRules{OSName: "Rocky Linux", OSMajor: 9}, Scope: "rocky:9"},
	{Match: config.MatchRules{OSName: "Rocky Linux", OSMajor: 10}, Scope: "rocky:10"},
	{Match: config.MatchRules{OSName: "AlmaLinux", OSMajor: 9}, Scope: "alma:9"},
	{Match: config.MatchRules{OSName: "AlmaLinux", OSMajor: 10}, Scope: "alma:10"},
	{Match: config.MatchRules{OSName: "Red Hat Enterprise Linux", OSMajor: 9}, Scope: "rhel:9"},
	{Match: config.MatchRules{OSName: "Red Hat Enterprise Linux", OSMajor: 10}, Scope: "rhel:10"},
}

func (s *advisorySyncService) ResolveScopeKey(ctx context.Context, osFamily, osName, osVersion string, osMajor int32, arch string) (string, error) {
	mappings := s.config.AdvisorySync.ScopeMappings
	if len(mappings) == 0 {
		mappings = defaultScopeMappings
	}

	for _, mapping := range mappings {
		match := true
		if mapping.Match.OSFamily != "" && !strings.EqualFold(mapping.Match.OSFamily, osFamily) {
			match = false
		}
		if mapping.Match.OSName != "" && !strings.Contains(strings.ToLower(osName), strings.ToLower(mapping.Match.OSName)) {
			match = false
		}
		if mapping.Match.OSVersion != "" && !strings.Contains(strings.ToLower(osVersion), strings.ToLower(mapping.Match.OSVersion)) {
			match = false
		}
		if mapping.Match.OSMajor != 0 && mapping.Match.OSMajor != osMajor {
			match = false
		}
		if mapping.Match.Architecture != "" && !strings.EqualFold(mapping.Match.Architecture, arch) {
			match = false
		}
		if match {
			return mapping.Scope, nil
		}
	}
	return "", nil
}

func (s *advisorySyncService) RegisterScopeDemand(ctx context.Context, scopeKey string) error {
	if scopeKey == "" {
		return nil
	}

	// Inserts scope key in pending status if it doesn't exist (prevents clobbering existing metrics)
	err := s.queries.InsertAdvisoryScopeIfNotExists(ctx, db.InsertAdvisoryScopeIfNotExistsParams{
		ScopeKey: scopeKey,
		Status:   "pending",
	})
	if err != nil {
		return fmt.Errorf("failed to register scope demand: %w", err)
	}

	// Trigger sync periodic job
	if err := s.periodicJobManager.AddAdvisorySyncJob(ctx, scopeKey); err != nil {
		return fmt.Errorf("failed to add periodic advisory sync job: %w", err)
	}

	return nil
}

func (s *advisorySyncService) SyncScope(ctx context.Context, scopeKey string) error {
	// 1. Mark as syncing
	_, err := s.queries.UpdateAdvisoryScopeStatus(ctx, db.UpdateAdvisoryScopeStatusParams{
		ScopeKey:  scopeKey,
		Status:    "syncing",
		LastError: utils.None[string](),
	})
	if err != nil {
		return fmt.Errorf("failed to update scope status to syncing: %w", err)
	}

	// Helper to update error status on failure
	handleFailure := func(syncErr error) error {
		errMsg := syncErr.Error()
		_, _ = s.queries.UpdateAdvisoryScopeStatus(ctx, db.UpdateAdvisoryScopeStatusParams{
			ScopeKey:  scopeKey,
			Status:    "failed",
			LastError: utils.Some(errMsg),
		})
		return syncErr
	}

	// 2. Fetch manifest.json
	manifestURL := strings.TrimSuffix(s.config.AdvisorySync.BaseURL, "/") + "/manifest.json"
	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return handleFailure(fmt.Errorf("failed to create manifest request: %w", err))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return handleFailure(fmt.Errorf("failed to fetch manifest: %w", err))
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return handleFailure(fmt.Errorf("manifest fetch returned status: %d", resp.StatusCode))
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return handleFailure(fmt.Errorf("failed to decode manifest: %w", err))
	}

	// 3. Find scope detail
	var detail *ScopeDetail
	for i := range manifest.Scopes {
		if manifest.Scopes[i].Key == scopeKey {
			detail = &manifest.Scopes[i]
			break
		}
	}
	if detail == nil {
		return handleFailure(fmt.Errorf("scope %q not found in manifest", scopeKey))
	}

	// Get current db record for previous files clean up
	current, err := s.queries.GetAdvisoryScope(ctx, scopeKey)
	hasPrevious := err == nil && current.Sha256.IsPresent() && current.LocalPath.IsPresent()

	// 4. Download if hash changed or local file doesn't exist
	destDir := s.config.AdvisorySync.StorageDir
	if err := s.fs.MkdirAll(destDir, 0755); err != nil {
		return handleFailure(fmt.Errorf("failed to create storage directory: %w", err))
	}

	destFilename := fmt.Sprintf("advisories-%s.db", detail.Sha256)
	destPath := filepath.Join(destDir, destFilename)

	exists, err := afero.Exists(s.fs, destPath)
	needsDownload := !exists || err != nil || current.Sha256.UnwrapOr("") != detail.Sha256

	if needsDownload {
		downloadURL := detail.URL
		if downloadURL == "" {
			downloadURL = strings.TrimSuffix(s.config.AdvisorySync.BaseURL, "/") + "/" + detail.Path
		}

		dlReq, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			return handleFailure(fmt.Errorf("failed to create download request: %w", err))
		}

		dlResp, err := s.httpClient.Do(dlReq)
		if err != nil {
			return handleFailure(fmt.Errorf("failed to download scope database: %w", err))
		}
		defer dlResp.Body.Close() //nolint:errcheck

		if dlResp.StatusCode != http.StatusOK {
			return handleFailure(fmt.Errorf("download scope database returned status: %d", dlResp.StatusCode))
		}

		// Create temp file for writing and hashing
		tmpFile, err := afero.TempFile(s.fs, destDir, "advisory-dl-*")
		if err != nil {
			return handleFailure(fmt.Errorf("failed to create temp file: %w", err))
		}
		tmpName := tmpFile.Name()
		defer func() {
			_ = s.fs.Remove(tmpName) // clean up temp file if rename was not successful
		}()

		hasher := sha256.New()
		writer := io.MultiWriter(tmpFile, hasher)

		if _, err := io.Copy(writer, dlResp.Body); err != nil {
			tmpFile.Close() // nolint:errcheck
			return handleFailure(fmt.Errorf("failed to write database file: %w", err))
		}
		if err := tmpFile.Close(); err != nil {
			return handleFailure(fmt.Errorf("failed to close database file: %w", err))
		}

		// Verify hash
		sum := hex.EncodeToString(hasher.Sum(nil))
		if sum != detail.Sha256 {
			return handleFailure(fmt.Errorf("checksum mismatch: expected %s, got %s", detail.Sha256, sum))
		}

		// Atomically move/rename file
		if err := s.fs.Rename(tmpName, destPath); err != nil {
			return handleFailure(fmt.Errorf("failed to rename temp file to destination: %w", err))
		}
	}

	// 5. Open SQLite database to verify it's valid
	sqliteDB, err := sql.Open("sqlite", destPath)
	if err != nil {
		return handleFailure(fmt.Errorf("failed to open downloaded SQLite database: %w", err))
	}
	defer sqliteDB.Close() //nolint:errcheck

	var rowCount int32
	err = sqliteDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM advisories").Scan(&rowCount)
	if err != nil {
		return handleFailure(fmt.Errorf("failed to verify advisories table in SQLite: %w", err))
	}

	// 6. Clean up previous file if hash changed and we have a new file
	if hasPrevious && current.Sha256.UnwrapOr("") != detail.Sha256 && current.LocalPath.UnwrapOr("") != destPath {
		_ = s.fs.Remove(current.LocalPath.UnwrapOr(""))
	}

	// 7. Update status to synced
	now := time.Now().UTC()
	nextRefresh := now.Add(s.config.AdvisorySync.RefreshInterval)
	_, err = s.queries.UpsertAdvisoryScope(ctx, db.UpsertAdvisoryScopeParams{
		ScopeKey:      scopeKey,
		Status:        "synced",
		LastSyncAt:    pgtype.Timestamptz{Time: now, Valid: true},
		LastSuccessAt: pgtype.Timestamptz{Time: now, Valid: true},
		LastError:     utils.None[string](),
		AdvisoryCount: rowCount,
		Sha256:        utils.Some(detail.Sha256),
		SizeBytes:     detail.SizeBytes,
		LocalPath:     utils.Some(destPath),
		NextRefreshAt: pgtype.Timestamptz{Time: nextRefresh, Valid: true},
	})
	if err != nil {
		return handleFailure(fmt.Errorf("failed to save synced scope metadata: %w", err))
	}

	return nil
}

func (s *advisorySyncService) TriggerManualSync(ctx context.Context, scopeKey string) error {
	riverClient, err := do.Invoke[*river.Client[pgx.Tx]](s.injector)
	if err != nil {
		return fmt.Errorf("failed to resolve river client: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	// Mark status as pending (upsert preserves other synced fields if row exists, registers if missing)
	_, err = db.New(tx).UpsertAdvisoryScopeStatus(ctx, db.UpsertAdvisoryScopeStatusParams{
		ScopeKey:  scopeKey,
		Status:    "pending",
		LastError: utils.None[string](),
	})
	if err != nil {
		return fmt.Errorf("failed to update scope status to pending: %w", err)
	}

	// Insert immediately to run now. If a job is already available, pending, running, or scheduled, skip.
	_, err = riverClient.InsertTx(ctx, tx, AdvisorySyncArgs{ScopeKey: scopeKey}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to insert manual sync job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if err := s.periodicJobManager.AddAdvisorySyncJob(ctx, scopeKey); err != nil {
		utils.GetLogger(ctx).
			ErrorContext(ctx, "failed to add periodic advisory sync job after manual sync trigger", "scope_key", scopeKey, "error", err)
	}

	return nil
}

func (s *advisorySyncService) GetScopeStatuses(ctx context.Context) ([]AdvisoryScopeStatus, error) {
	rows, err := s.queries.ListAdvisoryScopeStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list scope stats: %w", err)
	}

	statuses := make([]AdvisoryScopeStatus, len(rows))
	for i, r := range rows {
		statuses[i] = AdvisoryScopeStatus{
			ScopeKey:       r.ScopeKey,
			Status:         r.Status,
			LastSyncAt:     optTime(r.LastSyncAt),
			LastSuccessAt:  optTime(r.LastSuccessAt),
			LastError:      optString(r.LastError),
			AdvisoryCount:  r.AdvisoryCount,
			Sha256:         optString(r.Sha256),
			SizeBytes:      r.SizeBytes,
			LocalPath:      optString(r.LocalPath),
			NextRefreshAt:  optTime(r.NextRefreshAt),
			HostUsageCount: r.HostCount,
		}
	}
	return statuses, nil
}

func (s *advisorySyncService) GetOverview(ctx context.Context) (AdvisoryOverview, error) {
	rows, err := s.queries.ListAdvisoryScopes(ctx)
	if err != nil {
		return AdvisoryOverview{}, fmt.Errorf("failed to list scopes for overview: %w", err)
	}

	var totalAdvisories int64
	var syncedCount int32
	for _, r := range rows {
		totalAdvisories += int64(r.AdvisoryCount)
		if r.Status == "synced" {
			syncedCount++
		}
	}

	return AdvisoryOverview{
		TotalAdvisories: totalAdvisories,
		TotalScopes:     int32(len(rows)),
		SyncedScopes:    syncedCount,
	}, nil
}

func optTime(o pgtype.Timestamptz) *time.Time {
	if o.Valid {
		t := o.Time.UTC()
		return &t
	}
	return nil
}

func optString(o utils.Option[string]) *string {
	if o.IsPresent() {
		val := o.Unwrap()
		return &val
	}
	return nil
}
