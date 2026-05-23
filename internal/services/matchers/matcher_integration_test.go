package matchers_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/services/matchers"
	apitesting "go.patchbase.net/server/internal/testing"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMatcher_MatchSnapshot_Execution(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()

	// Seed host
	hostID := "h_test_matcher"
	snapshotID := "snap_test_matcher"

	dbConn := backend.DB()
	_, err := dbConn.Exec(ctx, `
		INSERT INTO hosts (id, hostname, os_family, os_name, os_version, os_major, architecture, onboarding_mode, approval_status, status)
		VALUES ($1, 'test-host', 'rpm', 'Rocky Linux', '9.3', 9, 'x86_64', 'agent', 'approved', 'active')
	`, hostID)
	require.NoError(t, err)

	// Build AgentSnapshot protobuf
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "test-host",
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
	payload, err := proto.Marshal(snap)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, $3, $4, '5.14.0', $5, false)
	`, snapshotID, hostID, pgtype.Timestamptz{Time: time.Now(), Valid: true}, payload, pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true})
	require.NoError(t, err)

	// Seed product stream and advisory
	_, err = dbConn.Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active')
	`)
	require.NoError(t, err)

	advisoryID := "RLSA-2023:9999"
	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ($1, 'rocky_errata_api', '9999', 'rocky', 'security', 'critical', 'Critical openssl vulnerability', 'vendor_db', true)
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ($1, 'rocky:9-baseos')
	`, advisoryID)
	require.NoError(t, err)

	// Affected package rule: openssl < 3.0.7-2.el9
	_, err = dbConn.Exec(ctx, `
		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, evidence_tier)
		VALUES ('rule_1', $1, 'rocky:9-baseos', 'openssl', '< 0:3.0.7-2.el9', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Fixed package: openssl-3.0.7-2.el9
	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_1', $1, 'rocky:9-baseos', 'openssl', 0, '3.0.7', '2.el9', 'x86_64', 'openssl-0:3.0.7-2.el9.x86_64', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Match snapshot
	matcher := do.MustInvoke[matchers.Matcher](backend.Injector())
	res, err := matcher.MatchSnapshot(ctx, hostID, snapshotID)
	require.NoError(t, err)

	assert.Equal(t, hostID, res.HostID)
	assert.Equal(t, snapshotID, res.SnapshotID)
	assert.Equal(t, 1, res.DecisionCount)
	assert.Equal(t, "update_package", res.OverallAction)
}
