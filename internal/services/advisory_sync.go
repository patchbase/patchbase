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
	"strconv"
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
	"go.patchbase.net/server/internal/services/matchers"
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
	if err := s.periodicJobManager.AddAdvisorySyncJob(ctx, scopeKey, true); err != nil {
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

	if !isValidSha256Hex(detail.Sha256) {
		return handleFailure(fmt.Errorf("invalid sha256 checksum format in manifest for scope %q: %q", scopeKey, detail.Sha256))
	}

	// Get current db record for previous files clean up
	current, err := s.queries.GetAdvisoryScope(ctx, scopeKey)
	currentExists := err == nil
	hasPrevious := currentExists && current.Sha256.IsPresent() && current.LocalPath.IsPresent()

	// 4. Download if hash changed or local file doesn't exist
	destDir := s.config.AdvisorySync.StorageDir
	if err := s.fs.MkdirAll(destDir, 0755); err != nil {
		return handleFailure(fmt.Errorf("failed to create storage directory: %w", err))
	}

	destFilename := fmt.Sprintf("advisories-%s.db", detail.Sha256)
	destPath := filepath.Join(destDir, destFilename)

	exists, err := afero.Exists(s.fs, destPath)
	sameHash := currentExists && current.Sha256.UnwrapOr("") == detail.Sha256
	hasSuccessfulImportForHash := sameHash && current.LastSuccessAt.Valid
	needsDownload := !exists || err != nil || !sameHash

	if !needsDownload && hasSuccessfulImportForHash {
		// If this exact hash was successfully imported before, skip re-import and just rematch hosts.
		matcherSvc, err := do.Invoke[matchers.Matcher](s.injector)
		if err != nil {
			return handleFailure(fmt.Errorf("failed to resolve Matcher service: %w", err))
		}
		if err := matcherSvc.MatchHostsForScope(ctx, scopeKey); err != nil {
			return handleFailure(fmt.Errorf("failed to match hosts for scope: %w", err))
		}

		now := time.Now().UTC()
		nextRefresh := now.Add(s.config.AdvisorySync.RefreshInterval)
		_, err = s.queries.UpsertAdvisoryScope(ctx, db.UpsertAdvisoryScopeParams{
			ScopeKey:      scopeKey,
			Status:        "synced",
			LastSyncAt:    pgtype.Timestamptz{Time: now, Valid: true},
			LastSuccessAt: pgtype.Timestamptz{Time: now, Valid: true},
			LastError:     utils.None[string](),
			AdvisoryCount: current.AdvisoryCount,
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

		// Download to a staging directory first, then atomically replace the destination on success.
		stageDir := filepath.Join(destDir, ".tmp")
		if err := s.fs.MkdirAll(stageDir, 0755); err != nil {
			return handleFailure(fmt.Errorf("failed to create staging directory: %w", err))
		}

		// Create temp file for writing and hashing
		tmpFile, err := afero.TempFile(s.fs, stageDir, "advisory-dl-*")
		if err != nil {
			return handleFailure(fmt.Errorf("failed to create temp file: %w", err))
		}
		tmpName := tmpFile.Name()
		defer func() {
			_ = s.fs.Remove(tmpName) // clean up temp file if rename was not successful
		}()

		hasher := sha256.New()
		writer := io.MultiWriter(tmpFile, hasher)

		writtenBytes, err := io.Copy(writer, dlResp.Body)
		if err != nil {
			tmpFile.Close() // nolint:errcheck
			return handleFailure(fmt.Errorf("failed to write database file: %w", err))
		}
		if writtenBytes == 0 {
			tmpFile.Close() // nolint:errcheck
			return handleFailure(fmt.Errorf("downloaded database file is empty"))
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

	// 5.5 Ingest SQLite records into Postgres
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return handleFailure(fmt.Errorf("failed to start postgres ingestion transaction: %w", err))
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	mappings := s.config.AdvisorySync.ScopeMappings
	if len(mappings) == 0 {
		mappings = defaultScopeMappings
	}
	if err := ImportAdvisoryDB(ctx, tx, db.New(tx), sqliteDB, scopeKey, mappings); err != nil {
		return handleFailure(fmt.Errorf("failed to import advisory database: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return handleFailure(fmt.Errorf("failed to commit postgres ingestion transaction: %w", err))
	}

	// 5.6 Match hosts under this scope key against updated advisory records
	matcherSvc, err := do.Invoke[matchers.Matcher](s.injector)
	if err != nil {
		return handleFailure(fmt.Errorf("failed to resolve Matcher service: %w", err))
	}
	if err := matcherSvc.MatchHostsForScope(ctx, scopeKey); err != nil {
		return handleFailure(fmt.Errorf("failed to match hosts for scope: %w", err))
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

	if err := s.periodicJobManager.AddAdvisorySyncJob(ctx, scopeKey, false); err != nil {
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

func getMajorVersionFromScopeKey(scopeKey string, mappings []config.ScopeMapping) int32 {
	if len(mappings) == 0 {
		mappings = defaultScopeMappings
	}
	for _, m := range mappings {
		if m.Scope == scopeKey {
			if m.Match.OSMajor != 0 {
				return m.Match.OSMajor
			}
			if m.Match.OSVersion != "" {
				parts := strings.Split(m.Match.OSVersion, ".")
				if len(parts) > 0 {
					if major, err := strconv.Atoi(parts[0]); err == nil {
						return int32(major)
					}
				}
			}
		}
	}
	parts := strings.Split(scopeKey, ":")
	if len(parts) == 2 {
		if major, err := strconv.Atoi(parts[1]); err == nil {
			return int32(major)
		}
	}
	return 0
}

func ImportAdvisoryDB(ctx context.Context, tx pgx.Tx, q *db.Queries, sqliteDB *sql.DB, scopeKey string, mappings []config.ScopeMapping) error {
	// 1. Fetch product streams from SQLite
	psRows, err := sqliteDB.QueryContext(ctx, "SELECT id, vendor, distro_family, distro_name, major_version, minor_version, architecture, repo_family, repo_id_pattern, cpe, status FROM product_streams")
	if err != nil {
		return fmt.Errorf("query product streams: %w", err)
	}
	defer func() { _ = psRows.Close() }()

	var streamIDs []string
	var productStreams []db.UpsertProductStreamParams
	for psRows.Next() {
		var id, vendor, distroFamily, distroName, repoFamily, status string
		var majorVersion int32
		var minorVersion, architecture, repoIDPattern, cpe *string
		if err := psRows.Scan(&id, &vendor, &distroFamily, &distroName, &majorVersion, &minorVersion, &architecture, &repoFamily, &repoIDPattern, &cpe, &status); err != nil {
			return fmt.Errorf("scan product stream: %w", err)
		}
		streamIDs = append(streamIDs, id)
		productStreams = append(productStreams, db.UpsertProductStreamParams{
			ID:            id,
			Vendor:        vendor,
			DistroFamily:  distroFamily,
			DistroName:    distroName,
			MajorVersion:  majorVersion,
			MinorVersion:  optFromPtr(minorVersion),
			Architecture:  optFromPtr(architecture),
			RepoFamily:    repoFamily,
			RepoIDPattern: optFromPtr(repoIDPattern),
			Cpe:           optFromPtr(cpe),
			Status:        status,
		})
	}
	if err := psRows.Err(); err != nil {
		return fmt.Errorf("product streams rows: %w", err)
	}

	var cleanUpStreamIDs []string
	parts := strings.Split(scopeKey, ":")
	if len(parts) == 2 {
		vendor := parts[0]
		if vendor == "rhel" {
			vendor = "redhat"
		}
		major := getMajorVersionFromScopeKey(scopeKey, mappings)

		existingIDs, err := q.ListProductStreamIDsByVendorAndVersion(ctx, db.ListProductStreamIDsByVendorAndVersionParams{
			Vendor:       vendor,
			MajorVersion: major,
		})
		if err == nil {
			cleanUpStreamIDs = append(cleanUpStreamIDs, existingIDs...)
		}
	}

	seenIDs := make(map[string]bool)
	for _, id := range cleanUpStreamIDs {
		seenIDs[id] = true
	}
	for _, id := range streamIDs {
		if !seenIDs[id] {
			cleanUpStreamIDs = append(cleanUpStreamIDs, id)
			seenIDs[id] = true
		}
	}

	// 2. Clear matching stream data in Postgres
	if len(cleanUpStreamIDs) > 0 {
		if err := q.DeleteAdvisoryReferencesByStreamIDs(ctx, cleanUpStreamIDs); err != nil {
			return fmt.Errorf("delete advisory references: %w", err)
		}
		if err := q.DeleteAdvisoryProductStreamsByStreamIDs(ctx, cleanUpStreamIDs); err != nil {
			return fmt.Errorf("delete advisory product streams: %w", err)
		}
		if err := q.DeleteAffectedPackageRulesByStreamIDs(ctx, cleanUpStreamIDs); err != nil {
			return fmt.Errorf("delete affected package rules: %w", err)
		}
		if err := q.DeleteFixedPackagesByStreamIDs(ctx, cleanUpStreamIDs); err != nil {
			return fmt.Errorf("delete fixed packages: %w", err)
		}
		if err := q.DeleteProductStreamsByIDs(ctx, cleanUpStreamIDs); err != nil {
			return fmt.Errorf("delete product streams: %w", err)
		}
		if err := q.DeleteAdvisoriesWithoutStreams(ctx); err != nil {
			return fmt.Errorf("delete advisories without streams: %w", err)
		}
	}

	// 3. Insert product streams
	for _, ps := range productStreams {
		if err := q.UpsertProductStream(ctx, ps); err != nil {
			return fmt.Errorf("upsert product stream %s: %w", ps.ID, err)
		}
	}

	// 4. Fetch and insert advisories from SQLite
	advRows, err := sqliteDB.QueryContext(ctx, "SELECT id, source_system, raw_source_id, source_url, vendor, advisory_type, severity, summary, description, published_at, updated_at, evidence_tier, is_security FROM advisories")
	if err != nil {
		return fmt.Errorf("query advisories: %w", err)
	}
	defer func() { _ = advRows.Close() }()

	for advRows.Next() {
		var id, sourceSystem, rawSourceID, vendor, advisoryType, evidenceTier string
		var sourceURL, severity, summary, description, publishedAt, updatedAt *string
		var isSecurity bool
		if err := advRows.Scan(&id, &sourceSystem, &rawSourceID, &sourceURL, &vendor, &advisoryType, &severity, &summary, &description, &publishedAt, &updatedAt, &evidenceTier, &isSecurity); err != nil {
			return fmt.Errorf("scan advisory: %w", err)
		}
		err = q.UpsertAdvisory(ctx, db.UpsertAdvisoryParams{
			ID:           id,
			SourceSystem: sourceSystem,
			RawSourceID:  rawSourceID,
			SourceUrl:    optFromPtr(sourceURL),
			Vendor:       vendor,
			AdvisoryType: advisoryType,
			Severity:     optFromPtr(severity),
			Summary:      optFromPtr(summary),
			Description:  optFromPtr(description),
			PublishedAt:  optFromPtr(publishedAt),
			UpdatedAt:    optFromPtr(updatedAt),
			EvidenceTier: evidenceTier,
			IsSecurity:   isSecurity,
		})
		if err != nil {
			return fmt.Errorf("upsert advisory %s: %w", id, err)
		}
	}
	if err := advRows.Err(); err != nil {
		return fmt.Errorf("advisories rows: %w", err)
	}

	// 5. Fetch and insert advisory references
	refRows, err := sqliteDB.QueryContext(ctx, "SELECT id, advisory_id, ref_type, ref_value, severity_vendor, severity_cvss, title, url FROM advisory_references")
	if err != nil {
		return fmt.Errorf("query advisory references: %w", err)
	}
	defer func() { _ = refRows.Close() }()

	for refRows.Next() {
		var id, advisoryID, refType, refValue string
		var severityVendor, title, url *string
		var severityCvss *float64
		if err := refRows.Scan(&id, &advisoryID, &refType, &refValue, &severityVendor, &severityCvss, &title, &url); err != nil {
			return fmt.Errorf("scan advisory reference: %w", err)
		}
		err = q.InsertAdvisoryReference(ctx, db.InsertAdvisoryReferenceParams{
			ID:             id,
			AdvisoryID:     advisoryID,
			RefType:        refType,
			RefValue:       refValue,
			SeverityVendor: optFromPtr(severityVendor),
			SeverityCvss:   severityCvss,
			Title:          optFromPtr(title),
			Url:            optFromPtr(url),
		})
		if err != nil {
			return fmt.Errorf("insert advisory reference %s: %w", id, err)
		}
	}
	if err := refRows.Err(); err != nil {
		return fmt.Errorf("advisory references rows: %w", err)
	}

	// 6. Fetch and insert advisory product stream links
	linkRows, err := sqliteDB.QueryContext(ctx, "SELECT advisory_id, product_stream_id FROM advisory_product_streams")
	if err != nil {
		return fmt.Errorf("query advisory product streams: %w", err)
	}
	defer func() { _ = linkRows.Close() }()

	for linkRows.Next() {
		var advisoryID, productStreamID string
		if err := linkRows.Scan(&advisoryID, &productStreamID); err != nil {
			return fmt.Errorf("scan advisory product stream link: %w", err)
		}
		err = q.InsertAdvisoryProductStream(ctx, db.InsertAdvisoryProductStreamParams{
			AdvisoryID:      advisoryID,
			ProductStreamID: productStreamID,
		})
		if err != nil {
			return fmt.Errorf("insert advisory product stream %s -> %s: %w", advisoryID, productStreamID, err)
		}
	}
	if err := linkRows.Err(); err != nil {
		return fmt.Errorf("advisory product streams rows: %w", err)
	}

	// 7. Fetch and insert affected package rules
	ruleRows, err := sqliteDB.QueryContext(ctx, "SELECT id, advisory_id, product_stream_id, package_name, source_rpm, arch, epoch_constraint, version_constraint, release_constraint, rpm_evr_rule, context, evidence_tier FROM affected_package_rules")
	if err != nil {
		return fmt.Errorf("query affected package rules: %w", err)
	}
	defer func() { _ = ruleRows.Close() }()

	for ruleRows.Next() {
		var id, advisoryID, productStreamID, packageName, contextStr, evidenceTier string
		var sourceRPM, arch, epochConstraint, versionConstraint, releaseConstraint, rpmEvrRule *string
		if err := ruleRows.Scan(&id, &advisoryID, &productStreamID, &packageName, &sourceRPM, &arch, &epochConstraint, &versionConstraint, &releaseConstraint, &rpmEvrRule, &contextStr, &evidenceTier); err != nil {
			return fmt.Errorf("scan affected package rule: %w", err)
		}
		err = q.InsertAffectedPackageRule(ctx, db.InsertAffectedPackageRuleParams{
			ID:                id,
			AdvisoryID:        advisoryID,
			ProductStreamID:   productStreamID,
			PackageName:       packageName,
			SourceRpm:         optFromPtr(sourceRPM),
			Arch:              optFromPtr(arch),
			EpochConstraint:   optFromPtr(epochConstraint),
			VersionConstraint: optFromPtr(versionConstraint),
			ReleaseConstraint: optFromPtr(releaseConstraint),
			RpmEvrRule:        optFromPtr(rpmEvrRule),
			Context:           contextStr,
			EvidenceTier:      evidenceTier,
		})
		if err != nil {
			return fmt.Errorf("insert affected package rule %s: %w", id, err)
		}
	}
	if err := ruleRows.Err(); err != nil {
		return fmt.Errorf("affected package rules rows: %w", err)
	}

	// 8. Fetch and insert fixed packages
	fixRows, err := sqliteDB.QueryContext(ctx, "SELECT id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, source_rpm, repo_family, evidence_tier FROM fixed_packages")
	if err != nil {
		return fmt.Errorf("query fixed packages: %w", err)
	}
	defer func() { _ = fixRows.Close() }()

	for fixRows.Next() {
		var id, advisoryID, productStreamID, packageName, version, release, nevra, evidenceTier string
		var epoch int32
		var arch, sourceRPM, repoFamily *string
		if err := fixRows.Scan(&id, &advisoryID, &productStreamID, &packageName, &epoch, &version, &release, &arch, &nevra, &sourceRPM, &repoFamily, &evidenceTier); err != nil {
			return fmt.Errorf("scan fixed package: %w", err)
		}
		err = q.InsertFixedPackage(ctx, db.InsertFixedPackageParams{
			ID:              id,
			AdvisoryID:      advisoryID,
			ProductStreamID: productStreamID,
			PackageName:     packageName,
			Epoch:           epoch,
			Version:         version,
			Release:         release,
			Arch:            optFromPtr(arch),
			Nevra:           nevra,
			SourceRpm:       optFromPtr(sourceRPM),
			RepoFamily:      optFromPtr(repoFamily),
			EvidenceTier:    evidenceTier,
		})
		if err != nil {
			return fmt.Errorf("insert fixed package %s: %w", id, err)
		}
	}
	if err := fixRows.Err(); err != nil {
		return fmt.Errorf("fixed packages rows: %w", err)
	}

	return nil
}

func optFromPtr[T any](p *T) utils.Option[T] {
	if p == nil {
		return utils.None[T]()
	}
	return utils.Some(*p)
}

func isValidSha256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
