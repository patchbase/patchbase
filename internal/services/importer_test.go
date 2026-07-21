// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package services_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	db "go.patchbase.net/server/internal/sql"
	apitesting "go.patchbase.net/server/internal/testing"

	_ "modernc.org/sqlite"
)

func createTestSQLiteDB(t *testing.T) (*sql.DB, string) {
	tmpFile, err := os.CreateTemp("", "patchbase-test-sqlite-*.db")
	require.NoError(t, err)
	dbPath := tmpFile.Name()
	_ = tmpFile.Close() // close so sqlite driver can open it

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

func TestImporter_SyncPrunesStaleDataAndEmptyStreams(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()
	queries := db.New(backend.DB())

	// 1. Create a populated test SQLite DB
	sqliteDB, dbPath := createTestSQLiteDB(t)
	defer func() { _ = os.Remove(dbPath) }()
	defer func() { _ = sqliteDB.Close() }()

	// Seed one stream, advisory, reference, and rules
	_, err := sqliteDB.Exec(`
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active');

		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ('RLSA-2023:9999', 'rocky_errata_api', '9999', 'rocky', 'security', 'critical', 'Summary', 'vendor_db', 1);

		INSERT INTO advisory_references (id, advisory_id, ref_type, ref_value, url)
		VALUES ('ref_9999', 'RLSA-2023:9999', 'cve', 'CVE-2023-9999', 'http://cve.com');

		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ('RLSA-2023:9999', 'rocky:9-baseos');

		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, context, evidence_tier)
		VALUES ('rule_9999', 'RLSA-2023:9999', 'rocky:9-baseos', 'openssl', '< 0:3.0.7-2.el9', 'installed_package', 'vendor_db');
	`)
	require.NoError(t, err)

	// Ingest populated DB into Postgres
	tx, err := backend.DB().BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	err = apitesting.TestImportAdvisoryDB(ctx, tx, db.New(tx), sqliteDB, "rocky:9")
	require.NoError(t, err)
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify data exists in Postgres
	streams, err := queries.ListProductStreams(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, streams)

	advisories, err := queries.ListAdvisoriesByStreamIDs(ctx, []string{"rocky:9-baseos"})
	require.NoError(t, err)
	assert.Len(t, advisories, 1)

	var refCount int
	err = backend.DB().QueryRow(ctx, "SELECT COUNT(*) FROM advisory_references").Scan(&refCount)
	require.NoError(t, err)
	assert.Equal(t, 1, refCount)

	// 2. Perform sync with an empty SQLite DB
	emptySqliteDB, emptyDbPath := createTestSQLiteDB(t)
	defer func() { _ = os.Remove(emptyDbPath) }()
	defer func() { _ = emptySqliteDB.Close() }()

	// Ingest empty DB into Postgres
	tx2, err := backend.DB().BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx2.Rollback(ctx) }()
	err = apitesting.TestImportAdvisoryDB(ctx, tx2, db.New(tx2), emptySqliteDB, "rocky:9")
	require.NoError(t, err)
	err = tx2.Commit(ctx)
	require.NoError(t, err)

	// Verify all data is pruned from Postgres
	streams, err = queries.ListProductStreams(ctx)
	require.NoError(t, err)
	assert.Empty(t, streams)

	advisories, err = queries.ListAdvisoriesByStreamIDs(ctx, []string{"rocky:9-baseos"})
	require.NoError(t, err)
	assert.Empty(t, advisories)

	err = backend.DB().QueryRow(ctx, "SELECT COUNT(*) FROM advisory_references").Scan(&refCount)
	require.NoError(t, err)
	assert.Equal(t, 0, refCount)
}

func TestImporter_SyncPrunesStaleDataAndEmptyStreams_CodenameScope(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()
	queries := db.New(backend.DB())

	// 1. Create a populated test SQLite DB
	sqliteDB, dbPath := createTestSQLiteDB(t)
	defer func() { _ = os.Remove(dbPath) }()
	defer func() { _ = sqliteDB.Close() }()

	// Seed one stream, advisory, reference, and rules for a codename scope (ubuntu:jammy)
	_, err := sqliteDB.Exec(`
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('ubuntu:jammy-main', 'ubuntu', 'deb', 'Ubuntu', 22, 'main', 'active');

		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ('USN-9999-1', 'ubuntu_security_tracker', '9999', 'ubuntu', 'security', 'critical', 'Summary', 'vendor_db', 1);

		INSERT INTO advisory_references (id, advisory_id, ref_type, ref_value, url)
		VALUES ('ref_ubuntu_9999', 'USN-9999-1', 'cve', 'CVE-2023-9999', 'http://cve.com');

		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ('USN-9999-1', 'ubuntu:jammy-main');

		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, context, evidence_tier)
		VALUES ('rule_ubuntu_9999', 'USN-9999-1', 'ubuntu:jammy-main', 'openssl', '< 0:3.0.2-0ubuntu1.15', 'installed_package', 'vendor_db');
	`)
	require.NoError(t, err)

	// Ingest populated DB into Postgres under the "ubuntu:jammy" codename scope
	tx, err := backend.DB().BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	err = apitesting.TestImportAdvisoryDB(ctx, tx, db.New(tx), sqliteDB, "ubuntu:jammy")
	require.NoError(t, err)
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify data exists in Postgres
	streams, err := queries.ListProductStreams(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, streams)

	advisories, err := queries.ListAdvisoriesByStreamIDs(ctx, []string{"ubuntu:jammy-main"})
	require.NoError(t, err)
	assert.Len(t, advisories, 1)

	var refCount int
	err = backend.DB().QueryRow(ctx, "SELECT COUNT(*) FROM advisory_references").Scan(&refCount)
	require.NoError(t, err)
	assert.Equal(t, 1, refCount)

	// 2. Perform sync with an empty SQLite DB for "ubuntu:jammy"
	emptySqliteDB, emptyDbPath := createTestSQLiteDB(t)
	defer func() { _ = os.Remove(emptyDbPath) }()
	defer func() { _ = emptySqliteDB.Close() }()

	// Ingest empty DB into Postgres under the same codename scope
	tx2, err := backend.DB().BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx2.Rollback(ctx) }()
	err = apitesting.TestImportAdvisoryDB(ctx, tx2, db.New(tx2), emptySqliteDB, "ubuntu:jammy")
	require.NoError(t, err)
	err = tx2.Commit(ctx)
	require.NoError(t, err)

	// Verify all data is pruned from Postgres (which proves the codename scope mapping resolved major version 22)
	streams, err = queries.ListProductStreams(ctx)
	require.NoError(t, err)
	assert.Empty(t, streams)

	advisories, err = queries.ListAdvisoriesByStreamIDs(ctx, []string{"ubuntu:jammy-main"})
	require.NoError(t, err)
	assert.Empty(t, advisories)

	err = backend.DB().QueryRow(ctx, "SELECT COUNT(*) FROM advisory_references").Scan(&refCount)
	require.NoError(t, err)
	assert.Equal(t, 0, refCount)
}
