// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.patchbase.net/server/internal/utils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHostVulnerableAndUpgradablePackages(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	createTokenRecorder := backend.HTTPPost(
		"/api/v1/hosts/tokens",
		`{"name":"fleet-token"}`,
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusCreated, createTokenRecorder.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createTokenRecorder.Body.Bytes(), &created))
	registrationToken := created["token"].(string)

	registerReq := &agentpb.RegisterHostRequest{
		RegistrationToken: registrationToken,
		Hostname:          "matching-host",
		MachineId:         "matching-machine",
		Metadata: &agentpb.RegisterHostMetadata{
			IpAddress:    "10.0.0.45",
			OsName:       "Rocky Linux",
			OsVersion:    "9.5",
			Architecture: "x86_64",
		},
	}
	registerReqBytes, err := proto.Marshal(registerReq)
	require.NoError(t, err)

	registerRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/register",
		registerReqBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
	)
	require.Equal(t, http.StatusCreated, registerRecorder.Code)

	var registered agentpb.RegisterHostResponse
	require.NoError(t, proto.Unmarshal(registerRecorder.Body.Bytes(), &registered))
	hostID := registered.HostId
	hostAccessToken := registered.HostAccessToken

	approveRecorder := backend.HTTPPost(
		fmt.Sprintf("/api/v1/hosts/%s/approve", hostID),
		"{}",
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, approveRecorder.Code)

	vulnEmptyRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, vulnEmptyRecorder.Code)
	var vulnEmpty []any
	require.NoError(t, json.Unmarshal(vulnEmptyRecorder.Body.Bytes(), &vulnEmpty))
	assert.Empty(t, vulnEmpty)

	upgEmptyRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/upgradable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, upgEmptyRecorder.Code)
	var upgEmpty []any
	require.NoError(t, json.Unmarshal(upgEmptyRecorder.Body.Bytes(), &upgEmpty))
	assert.Empty(t, upgEmpty)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:                   "matching-machine",
			Hostname:                    "matching-host",
			OsFamily:                    agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:                      "Rocky Linux",
			OsMajor:                     9,
			OsVersion:                   "9.5",
			Architecture:                agentpb.Architecture_ARCHITECTURE_X86_64,
			AvailablePackageUpdateCount: 2,
			IpAddresses:                 []string{"10.0.0.45"},
		},
		Packages: []*agentpb.Package{
			{
				Name:      "curl",
				Epoch:     0,
				Version:   "7.76.1",
				Release:   "14.el9",
				Arch:      "x86_64",
				SourceRpm: "curl-7.76.1-14.el9.src.rpm",
				Nevra:     "curl-0:7.76.1-14.el9.x86_64",
			},
		},
		UpgradablePackages: []*agentpb.Package{
			{
				Name:       "curl",
				Epoch:      0,
				Version:    "7.76.1",
				Release:    "15.el9",
				Arch:       "x86_64",
				RepoOrigin: "baseos",
				Nevra:      "curl-0:7.76.1-15.el9.x86_64",
			},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "kernel-5.14.0"},
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	ingestRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostAccessToken),
	)
	require.Equal(t, http.StatusAccepted, ingestRecorder.Code)

	vulnRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, vulnRecorder.Code)
	var vulnGroups []map[string]any
	require.NoError(t, json.Unmarshal(vulnRecorder.Body.Bytes(), &vulnGroups))
	assert.Empty(t, vulnGroups)

	upgRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/upgradable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, upgRecorder.Code)
	var upgGroups []map[string]any
	require.NoError(t, json.Unmarshal(upgRecorder.Body.Bytes(), &upgGroups))
	assert.NotEmpty(t, upgGroups)
}

func TestHostVulnerableAndUpgradablePackages_NonEmptyAndValidation(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	ctx := context.Background()
	_, err = backend.DB().Exec(ctx, `
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

	queries := db.New(backend.DB())
	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("Rocky9-Host"),
		MachineID:    utils.Some("mach-rocky-9"),
		Hostname:     utils.Some("rocky9-test"),
		IpAddress:    utils.Some("10.0.0.98"),
		OsName:       "Rocky Linux",
		OsVersion:    "9.3",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	hostToken := "pb_host_token987654321012"
	_, err = queries.InsertHostAccessToken(ctx, db.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    hostID,
		TokenHash: utils.SHA256(hostToken),
	})
	require.NoError(t, err)

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(time.Now()),
		Host: &agentpb.Host{
			MachineId:    "mach-rocky-9",
			Hostname:     "rocky9-test",
			OsFamily:     agentpb.OsFamily_OS_FAMILY_RPM,
			OsName:       "Rocky Linux",
			OsMajor:      9,
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
		Runtime: &agentpb.Runtime{KernelRunning: "5.14.0"},
	}
	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	ingestRecorder := backend.HTTPPostBytes(
		"/api/v1/agent/snapshots",
		payloadBytes,
		apitesting.WithHeader("Content-Type", "application/x-protobuf"),
		apitesting.WithBearerToken(hostToken),
	)
	require.Equal(t, http.StatusAccepted, ingestRecorder.Code)

	vulnRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, vulnRecorder.Code)

	var vulnGroups []map[string]any
	require.NoError(t, json.Unmarshal(vulnRecorder.Body.Bytes(), &vulnGroups))
	assert.NotEmpty(t, vulnGroups)
	assert.Equal(t, "openssl", vulnGroups[0]["family_label"])

	upgRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/upgradable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, upgRecorder.Code)

	var upgGroups []map[string]any
	require.NoError(t, json.Unmarshal(upgRecorder.Body.Bytes(), &upgGroups))
	assert.Empty(t, upgGroups)

	errRecorder := backend.HTTPGet(
		"/api/v1/hosts/non-existent-host-id/packages/vulnerable",
		apitesting.WithBearerToken(adminToken),
	)
	assert.Equal(t, http.StatusNotFound, errRecorder.Code)

	errUpgRecorder := backend.HTTPGet(
		"/api/v1/hosts/non-existent-host-id/packages/upgradable",
		apitesting.WithBearerToken(adminToken),
	)
	assert.Equal(t, http.StatusNotFound, errUpgRecorder.Code)
}

func TestHostVulnerablePackages_SeverityFallbackFromAdvisory(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	ctx := context.Background()
	queries := db.New(backend.DB())

	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("severity-fallback-host"),
		MachineID:    utils.Some("severity-fallback-machine"),
		Hostname:     utils.Some("severity-fallback-host"),
		IpAddress:    utils.Some("10.0.0.58"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)
	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	snapshotID := id.New("snap")
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, now(), $3, '5.15.0-156-generic', NULL, false)
	`, snapshotID, hostID, []byte{})
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES ('USN-7010-1', 'ubuntu_usn_api', '7010-1', 'ubuntu', 'security', 'high', 'Severity fallback advisory', 'vendor_db', true, now())
	`)
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-7010-1",
		PackageName:    "linux-image-5.15.0-156-generic",
		InstalledNevra: utils.Some("linux-image-5.15.0-156-generic-0:5.15.0-156.166.amd64"),
		FixedNevra:     utils.Some("linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.None[string](),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ReasonText:     utils.Some("a vendor fixed package is available but not installed"),
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	var groups []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &groups))
	require.NotEmpty(t, groups)
	assert.Equal(t, "Important", groups[0]["severity_label"])
}
func TestHostPackages_GroupingEdgeCases(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	ctx := context.Background()
	queries := db.New(backend.DB())

	hostID := id.New("h")
	_, err = queries.InsertAgentHost(ctx, db.InsertAgentHostParams{
		ID:           hostID,
		DisplayName:  utils.Some("edge-cases-host"),
		MachineID:    utils.Some("edge-cases-machine"),
		Hostname:     utils.Some("edge-cases-host"),
		IpAddress:    utils.Some("10.0.0.100"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	snapshotID := id.New("snap")
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, now(), $3, '5.15.0-156-generic', NULL, false)
	`, snapshotID, hostID, []byte{})
	require.NoError(t, err)

	// Advisory 1: Multiple CVEs, unknown severity.
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES ('USN-1000-1', 'ubuntu_usn_api', '1000-1', 'ubuntu', 'security', '', 'Unknown severity with multiple CVEs', 'vendor_db', true, '2026-05-20T10:00:00Z')
	`)
	require.NoError(t, err)
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisory_references (id, advisory_id, ref_type, ref_value, url)
		VALUES
			('ref1', 'USN-1000-1', 'cve', 'CVE-2026-10001', 'https://cve.org/10001'),
			('ref2', 'USN-1000-1', 'cve', 'CVE-2026-10002', 'https://cve.org/10002')
	`)
	require.NoError(t, err)

	// Advisory 2 & 3: Multiple advisories for same package, equal severities (critical) to test deterministic order
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES
			('USN-2000-2', 'ubuntu_usn_api', '2000-2', 'ubuntu', 'security', 'critical', 'Critical 2', 'vendor_db', true, '2026-05-21T10:00:00Z'),
			('USN-2000-1', 'ubuntu_usn_api', '2000-1', 'ubuntu', 'security', 'critical', 'Critical 1', 'vendor_db', true, '2026-05-21T10:00:00Z')
	`)
	require.NoError(t, err)

	// Advisory 4: reboot_host
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES ('USN-3000-1', 'ubuntu_usn_api', '3000-1', 'ubuntu', 'security', 'high', 'Reboot required', 'vendor_db', true, '2026-05-22T10:00:00Z')
	`)
	require.NoError(t, err)

	// Insert decision records
	// 1. Unknown severity
	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d1"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-1000-1",
		PackageName:    "unknown-pkg",
		InstalledNevra: utils.Some("unknown-pkg-0:1.0.0"),
		FixedNevra:     utils.Some("unknown-pkg-0:1.0.1"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.None[string](),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	// Duplicate decision row (same package, same advisory, different nevra)
	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d1-dup"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-1000-1",
		PackageName:    "unknown-pkg",
		InstalledNevra: utils.Some("unknown-pkg-0:1.0.0-alt"),
		FixedNevra:     utils.Some("unknown-pkg-0:1.0.1"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.None[string](),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	// 2. Multiple advisories per package & equal severity deterministic order
	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d2"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-2000-2",
		PackageName:    "critical-pkg",
		InstalledNevra: utils.Some("critical-pkg-0:2.0.0"),
		FixedNevra:     utils.Some("critical-pkg-0:2.0.1"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.Some("critical"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d3"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-2000-1",
		PackageName:    "critical-pkg",
		InstalledNevra: utils.Some("critical-pkg-0:2.0.0"),
		FixedNevra:     utils.Some("critical-pkg-0:2.0.1"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.Some("critical"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	// 3. reboot_host action (fixed-package-installed-pending-activation)
	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d4"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-3000-1",
		PackageName:    "reboot-pkg",
		InstalledNevra: utils.Some("reboot-pkg-0:3.0.0"),
		FixedNevra:     utils.Some("reboot-pkg-0:3.0.1"),
		Status:         "fixed_pending_activation",
		Action:         "reboot_host",
		Severity:       utils.Some("high"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_installed_pending_reboot",
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	// Get vulnerable packages
	recorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/packages/vulnerable", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	var groups []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &groups))

	// Assertions based on edge cases
	require.Len(t, groups, 3)

	// Group 1: reboot-pkg (reboot_host action has highest priority: 4)
	assert.Equal(t, "reboot", groups[0]["family_label"])
	assert.Equal(t, "Reboot required", groups[0]["action_label"])
	advisories0 := groups[0]["advisories"].([]any)
	require.Len(t, advisories0, 1)

	// Group 2: critical-pkg (update_package action: priority 2, but severity critical: priority 4)
	assert.Equal(t, "critical", groups[1]["family_label"])
	assert.Equal(t, "Critical", groups[1]["severity_label"])
	advisories1 := groups[1]["advisories"].([]any)
	require.Len(t, advisories1, 2)
	// USN-2000-1 should come before USN-2000-2 (deterministic sort by ID when updated_at and severity are equal)
	assert.Equal(t, "USN-2000-1", advisories1[0].(map[string]any)["advisory_id"])
	assert.Equal(t, "USN-2000-2", advisories1[1].(map[string]any)["advisory_id"])

	// Group 3: unknown-pkg (unknown severity: priority 0)
	assert.Equal(t, "unknown", groups[2]["family_label"])
	assert.Equal(t, "Unknown", groups[2]["severity_label"])
	advisories2 := groups[2]["advisories"].([]any)
	require.Len(t, advisories2, 1)
	adv2 := advisories2[0].(map[string]any)
	cves := adv2["cves"].([]any)
	require.Len(t, cves, 2) // Multiple CVEs
	assert.Equal(t, "CVE-2026-10001", cves[0].(map[string]any)["id"])

	// Duplicate decision row: Check if we have two items in the advisory
	items := adv2["items"].([]any)
	require.Len(t, items, 2)
}
