package matchers_test

import (
	"context"
	"sync"
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

func TestMatcher_MatchSnapshot_APT_Execution(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()

	// Seed host
	hostID := "h_test_matcher_apt"
	snapshotID := "snap_test_matcher_apt"

	dbConn := backend.DB()
	_, err := dbConn.Exec(ctx, `
		INSERT INTO hosts (id, hostname, os_family, os_name, os_version, os_major, architecture, onboarding_mode, approval_status, status)
		VALUES ($1, 'test-host-apt', 'apt', 'Ubuntu', '22.04', 22, 'x86_64', 'agent', 'approved', 'active')
	`, hostID)
	require.NoError(t, err)

	// Build AgentSnapshot protobuf
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "test-host-apt",
			OsName:       "Ubuntu",
			OsVersion:    "22.04",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "openssl",
				Epoch:   0,
				Version: "3.0.2",
				Release: "0ubuntu1",
				Arch:    "amd64",
				Nevra:   "openssl-0:3.0.2-0ubuntu1.amd64",
			},
		},
		Repos: []*agentpb.Repo{
			{
				RepoId:  "main",
				Enabled: true,
			},
		},
		Runtime: &agentpb.Runtime{
			KernelRunning: "5.15.0-91-generic",
		},
	}
	payload, err := proto.Marshal(snap)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, $3, $4, '5.15.0-91-generic', $5, false)
	`, snapshotID, hostID, pgtype.Timestamptz{Time: time.Now(), Valid: true}, payload, pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true})
	require.NoError(t, err)

	// Seed product stream and advisory
	_, err = dbConn.Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('ubuntu:22-main', 'ubuntu', 'apt', 'Ubuntu', 22, 'main', 'active')
	`)
	require.NoError(t, err)

	advisoryID := "USN-5999-1"
	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ($1, 'ubuntu_usn_api', '5999-1', 'ubuntu', 'security', 'critical', 'Critical openssl vulnerability', 'vendor_db', true)
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ($1, 'ubuntu:22-main')
	`, advisoryID)
	require.NoError(t, err)

	// Affected package rule: openssl < 3.0.2-0ubuntu1.1
	_, err = dbConn.Exec(ctx, `
		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, evidence_tier)
		VALUES ('rule_apt_1', $1, 'ubuntu:22-main', 'openssl', '< 0:3.0.2-0ubuntu1.1', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Fixed package: openssl-3.0.2-0ubuntu1.1
	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_apt_1', $1, 'ubuntu:22-main', 'openssl', 0, '3.0.2', '0ubuntu1.1', 'amd64', 'openssl-0:3.0.2-0ubuntu1.1.amd64', 'vendor_db')
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

func TestMatcher_MatchSnapshot_APT_KernelReboot(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()

	// Seed host
	hostID := "h_test_matcher_apt_kernel"
	snapshotID := "snap_test_matcher_apt_kernel"

	dbConn := backend.DB()
	_, err := dbConn.Exec(ctx, `
		INSERT INTO hosts (id, hostname, os_family, os_name, os_version, os_major, architecture, onboarding_mode, approval_status, status)
		VALUES ($1, 'test-host-apt-kernel', 'apt', 'Ubuntu', '22.04', 22, 'x86_64', 'agent', 'approved', 'active')
	`, hostID)
	require.NoError(t, err)

	// Build AgentSnapshot protobuf
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "test-host-apt-kernel",
			OsName:       "Ubuntu",
			OsVersion:    "22.04",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "linux-image-5.15.0-91-generic",
				Epoch:   0,
				Version: "5.15.0",
				Release: "91.101",
				Arch:    "amd64",
				Nevra:   "linux-image-5.15.0-91-generic-0:5.15.0-91.101.amd64",
			},
		},
		Repos: []*agentpb.Repo{
			{
				RepoId:  "main",
				Enabled: true,
			},
		},
		Runtime: &agentpb.Runtime{
			KernelRunning: "5.15.0-90-generic", // Running kernel version is older (90 < 91)
		},
	}
	payload, err := proto.Marshal(snap)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, $3, $4, '5.15.0-90-generic', $5, false)
	`, snapshotID, hostID, pgtype.Timestamptz{Time: time.Now(), Valid: true}, payload, pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true})
	require.NoError(t, err)

	// Seed product stream and advisory
	_, err = dbConn.Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('ubuntu:22-main', 'ubuntu', 'apt', 'Ubuntu', 22, 'main', 'active')
	`)
	require.NoError(t, err)

	advisoryID := "USN-5999-2"
	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ($1, 'ubuntu_usn_api', '5999-2', 'ubuntu', 'security', 'critical', 'Critical kernel vulnerability', 'vendor_db', true)
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ($1, 'ubuntu:22-main')
	`, advisoryID)
	require.NoError(t, err)

	// Fixed package: linux-image-5.15.0-91-generic version 5.15.0-91.101
	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_apt_k1', $1, 'ubuntu:22-main', 'linux-image-5.15.0-91-generic', 0, '5.15.0', '91.101', 'amd64', 'linux-image-5.15.0-91-generic-0:5.15.0-91.101.amd64', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Match snapshot
	matcher := do.MustInvoke[matchers.Matcher](backend.Injector())
	res, err := matcher.MatchSnapshot(ctx, hostID, snapshotID)
	require.NoError(t, err)

	assert.Equal(t, hostID, res.HostID)
	assert.Equal(t, snapshotID, res.SnapshotID)
	assert.Equal(t, 1, res.DecisionCount)
	assert.Equal(t, "reboot_host", res.OverallAction) // Expected action is reboot_host!
}

func TestMatcher_MatchSnapshot_APT_KernelNoReboot(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()

	// Seed host
	hostID := "h_test_matcher_apt_kernel_noreboot"
	snapshotID := "snap_test_matcher_apt_kernel_noreboot"

	dbConn := backend.DB()
	_, err := dbConn.Exec(ctx, `
		INSERT INTO hosts (id, hostname, os_family, os_name, os_version, os_major, architecture, onboarding_mode, approval_status, status)
		VALUES ($1, 'test-host-apt-kernel-noreboot', 'apt', 'Ubuntu', '22.04', 22, 'x86_64', 'agent', 'approved', 'active')
	`, hostID)
	require.NoError(t, err)

	// Build AgentSnapshot protobuf
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "test-host-apt-kernel-noreboot",
			OsName:       "Ubuntu",
			OsVersion:    "22.04",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "linux-image-5.15.0-91-generic",
				Epoch:   0,
				Version: "5.15.0",
				Release: "91.101",
				Arch:    "amd64",
				Nevra:   "linux-image-5.15.0-91-generic-0:5.15.0-91.101.amd64",
			},
		},
		Repos: []*agentpb.Repo{
			{
				RepoId:  "main",
				Enabled: true,
			},
		},
		Runtime: &agentpb.Runtime{
			KernelRunning: "5.15.0-91-generic", // Running kernel version matches installed package (91 = 91)
		},
	}
	payload, err := proto.Marshal(snap)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, $3, $4, '5.15.0-91-generic', $5, false)
	`, snapshotID, hostID, pgtype.Timestamptz{Time: time.Now(), Valid: true}, payload, pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true})
	require.NoError(t, err)

	// Seed product stream and advisory
	_, err = dbConn.Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('ubuntu:22-main', 'ubuntu', 'apt', 'Ubuntu', 22, 'main', 'active')
	`)
	require.NoError(t, err)

	advisoryID := "USN-5999-3"
	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ($1, 'ubuntu_usn_api', '5999-3', 'ubuntu', 'security', 'critical', 'Critical kernel vulnerability', 'vendor_db', true)
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ($1, 'ubuntu:22-main')
	`, advisoryID)
	require.NoError(t, err)

	// Fixed package: linux-image-5.15.0-91-generic version 5.15.0-91.101
	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_apt_k2', $1, 'ubuntu:22-main', 'linux-image-5.15.0-91-generic', 0, '5.15.0', '91.101', 'amd64', 'linux-image-5.15.0-91-generic-0:5.15.0-91.101.amd64', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Match snapshot
	matcher := do.MustInvoke[matchers.Matcher](backend.Injector())
	res, err := matcher.MatchSnapshot(ctx, hostID, snapshotID)
	require.NoError(t, err)

	assert.Equal(t, hostID, res.HostID)
	assert.Equal(t, snapshotID, res.SnapshotID)
	assert.Equal(t, 0, res.DecisionCount)
	assert.Equal(t, "none", res.OverallAction) // Expected action is none because running kernel matches fixed package version!
}

func TestMatcher_MatchSnapshot_APT_KernelReboot_Debian(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()

	// Seed host
	hostID := "h_test_matcher_apt_kernel_debian"
	snapshotID := "snap_test_matcher_apt_kernel_debian"

	dbConn := backend.DB()
	_, err := dbConn.Exec(ctx, `
		INSERT INTO hosts (id, hostname, os_family, os_name, os_version, os_major, architecture, onboarding_mode, approval_status, status)
		VALUES ($1, 'test-host-apt-kernel-debian', 'apt', 'Debian GNU/Linux', '12', 12, 'x86_64', 'agent', 'approved', 'active')
	`, hostID)
	require.NoError(t, err)

	// Build AgentSnapshot protobuf
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "test-host-apt-kernel-debian",
			OsName:       "Debian GNU/Linux",
			OsVersion:    "12",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "linux-image-6.1.0-18-amd64",
				Epoch:   0,
				Version: "6.1.76",
				Release: "1",
				Arch:    "amd64",
				Nevra:   "linux-image-6.1.0-18-amd64-0:6.1.76-1.amd64",
			},
			{
				Name:    "linux-image-6.1.0-19-amd64", // Newer kernel package is installed but not running
				Epoch:   0,
				Version: "6.1.76",
				Release: "2",
				Arch:    "amd64",
				Nevra:   "linux-image-6.1.0-19-amd64-0:6.1.76-2.amd64",
			},
		},
		Repos: []*agentpb.Repo{
			{
				RepoId:  "main",
				Enabled: true,
			},
		},
		Runtime: &agentpb.Runtime{
			KernelRunning: "6.1.0-18-amd64", // Running kernel version (18) is older than latest installed package (19)
		},
	}
	payload, err := proto.Marshal(snap)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, $3, $4, '6.1.0-18-amd64', $5, false)
	`, snapshotID, hostID, pgtype.Timestamptz{Time: time.Now(), Valid: true}, payload, pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true})
	require.NoError(t, err)

	// Seed product stream and advisory
	_, err = dbConn.Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('debian:12-main', 'debian', 'apt', 'Debian GNU/Linux', 12, 'main', 'active')
	`)
	require.NoError(t, err)

	advisoryID := "DSA-5999-1"
	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security)
		VALUES ($1, 'debian_dsa_api', '5999-1', 'debian', 'security', 'critical', 'Critical kernel vulnerability', 'vendor_db', true)
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ($1, 'debian:12-main')
	`, advisoryID)
	require.NoError(t, err)

	// Fixed package: linux-image-6.1.0-19-amd64 version 6.1.76-2
	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_apt_k3', $1, 'debian:12-main', 'linux-image-6.1.0-19-amd64', 0, '6.1.76', '2', 'amd64', 'linux-image-6.1.0-19-amd64-0:6.1.76-2.amd64', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Fixed package: linux-image-6.1.0-18-amd64 version 6.1.76-2
	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_apt_k4', $1, 'debian:12-main', 'linux-image-6.1.0-18-amd64', 0, '6.1.76', '2', 'amd64', 'linux-image-6.1.0-18-amd64-0:6.1.76-2.amd64', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	// Match snapshot
	matcher := do.MustInvoke[matchers.Matcher](backend.Injector())
	res, err := matcher.MatchSnapshot(ctx, hostID, snapshotID)
	require.NoError(t, err)

	assert.Equal(t, hostID, res.HostID)
	assert.Equal(t, snapshotID, res.SnapshotID)
	assert.Equal(t, 2, res.DecisionCount)
	assert.Equal(t, "reboot_host", res.OverallAction) // Expected action is reboot_host!
}

func TestMatcher_Concurrent_MatchSnapshot(t *testing.T) {
	backend := apitesting.NewBackend(t)
	ctx := context.Background()

	// Seed host
	hostID := "h_test_concurrent_matcher"
	snapshotID := "snap_test_concurrent_matcher"

	dbConn := backend.DB()
	_, err := dbConn.Exec(ctx, `
		INSERT INTO hosts (id, hostname, os_family, os_name, os_version, os_major, architecture, onboarding_mode, approval_status, status)
		VALUES ($1, 'test-concurrent-host', 'rpm', 'Rocky Linux', '9.3', 9, 'x86_64', 'agent', 'approved', 'active')
	`, hostID)
	require.NoError(t, err)

	// Build AgentSnapshot protobuf
	snap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			Hostname:     "test-concurrent-host",
			OsName:       "Rocky Linux",
			OsVersion:    "9.3",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{Name: "bash", Epoch: 0, Version: "5.1.8", Release: "7.el9_0", Arch: "x86_64", Nevra: "bash-0:5.1.8-7.el9_0.x86_64"},
		},
		Repos: []*agentpb.Repo{
			{RepoId: "baseos", Enabled: true},
		},
	}
	payloadBytes, err := proto.Marshal(snap)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, has_process_data)
		VALUES ($1, $2, now(), $3, '', false)
	`, snapshotID, hostID, payloadBytes)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO product_streams (id, vendor, distro_family, distro_name, major_version, repo_family, status)
		VALUES ('rocky:9-baseos', 'rocky', 'rpm', 'Rocky Linux', 9, 'baseos', 'active')
	`)
	require.NoError(t, err)

	// Seed advisory
	advisoryID := "adv_bash_concurrent"
	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, evidence_tier)
		VALUES ($1, 'rhsa', 'RHSA-2022:1234', 'Rocky', 'security', 'important', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO advisory_product_streams (advisory_id, product_stream_id)
		VALUES ($1, 'rocky:9-baseos')
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO fixed_packages (id, advisory_id, product_stream_id, package_name, epoch, version, release, arch, nevra, evidence_tier)
		VALUES ('fix_bash_concurrent', $1, 'rocky:9-baseos', 'bash', 0, '5.1.8', '9.el9', 'x86_64', 'bash-0:5.1.8-9.el9.x86_64', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	_, err = dbConn.Exec(ctx, `
		INSERT INTO affected_package_rules (id, advisory_id, product_stream_id, package_name, rpm_evr_rule, evidence_tier)
		VALUES ('rule_bash_concurrent', $1, 'rocky:9-baseos', 'bash', '< 0:5.1.8-9.el9', 'vendor_db')
	`, advisoryID)
	require.NoError(t, err)

	matcher := do.MustInvoke[matchers.Matcher](backend.Injector())

	// Run concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, 5)
	for range 5 {
		wg.Go(func() {
			_, err := matcher.MatchSnapshot(ctx, hostID, snapshotID)
			if err != nil {
				errCh <- err
			}
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	// Verify exact number of rows in DB
	var count int
	err = dbConn.QueryRow(ctx, `
		SELECT COUNT(*) FROM decision_records WHERE snapshot_id = $1
	`, snapshotID).Scan(&count)
	require.NoError(t, err)

	// It should leave EXACTLY ONE row because the others deleted what was there and overwrote,
	// or the lock safely serialized them.
	assert.Equal(t, 1, count, "should have exactly one decision record despite 5 concurrent matcher runs")
}
