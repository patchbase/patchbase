package testing

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/services"
	db "go.patchbase.net/server/internal/sql"
)

// TestImportAdvisoryDB wraps ImportAdvisoryDB for external testing.
func TestImportAdvisoryDB(ctx context.Context, tx pgx.Tx, q *db.Queries, sqliteDB *sql.DB, targetScope string) error {
	return services.ImportAdvisoryDB(ctx, tx, q, sqliteDB, targetScope, nil)
}

// TestCleanQuote wraps CleanQuote for external testing.
func TestCleanQuote(value string) string {
	return services.CleanQuote(value)
}

// TestCountAptPackageUpdates wraps CountAptPackageUpdates for external testing.
func TestCountAptPackageUpdates(output string) int32 {
	return services.CountAptPackageUpdates(output)
}

// TestCountRpmPackageUpdates wraps CountRpmPackageUpdates for external testing.
func TestCountRpmPackageUpdates(output string) int32 {
	return services.CountRpmPackageUpdates(output)
}

// TestParseUpgradablePackages wraps ParseUpgradablePackages for external testing.
func TestParseUpgradablePackages(osFamily string, output string) []*agentpb.Package {
	return services.ParseUpgradablePackages(osFamily, output)
}
