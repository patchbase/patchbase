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

func TestHostKernelPosture(t *testing.T) {
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
		DisplayName:  utils.Some("kernel-host"),
		MachineID:    utils.Some("kernel-machine"),
		Hostname:     utils.Some("kernel-host"),
		IpAddress:    utils.Some("10.0.0.47"),
		OsName:       "Ubuntu",
		OsVersion:    "22.04",
		Architecture: "x86_64",
	})
	require.NoError(t, err)

	_, err = queries.ApproveHostByID(ctx, hostID)
	require.NoError(t, err)

	emptyPostureRecorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/kernel-posture", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, emptyPostureRecorder.Code)
	var emptyPayload map[string]any
	require.NoError(t, json.Unmarshal(emptyPostureRecorder.Body.Bytes(), &emptyPayload))
	assert.Equal(t, "", emptyPayload["running_kernel"])
	assert.Equal(t, "", emptyPayload["latest_installed_kernel"])

	snapshot := &agentpb.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        timestamppb.New(time.Now().UTC()),
		Host: &agentpb.Host{
			MachineId:    "kernel-machine",
			Hostname:     "kernel-host",
			OsFamily:     agentpb.OsFamily_OS_FAMILY_APT,
			OsName:       "Ubuntu",
			OsMajor:      22,
			OsVersion:    "22.04",
			Architecture: agentpb.Architecture_ARCHITECTURE_X86_64,
		},
		Packages: []*agentpb.Package{
			{
				Name:    "linux-image-5.15.0-156-generic",
				Epoch:   0,
				Version: "5.15.0",
				Release: "156.166",
				Arch:    "amd64",
				Nevra:   "linux-image-5.15.0-156-generic-0:5.15.0-156.166.amd64",
			},
			{
				Name:    "linux-image-5.15.0-176-generic",
				Epoch:   0,
				Version: "5.15.0",
				Release: "176.186",
				Arch:    "amd64",
				Nevra:   "linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64",
			},
		},
		Runtime: &agentpb.Runtime{KernelRunning: "5.15.0-156-generic"},
	}

	payloadBytes, err := proto.Marshal(snapshot)
	require.NoError(t, err)

	snapshotID := id.New("snap")
	_, err = backend.DB().Exec(ctx, `
		INSERT INTO host_snapshots (id, host_id, collected_at, payload, running_kernel_nevra, boot_time, has_process_data)
		VALUES ($1, $2, now(), $3, '5.15.0-156-generic', NULL, false)
	`, snapshotID, hostID, payloadBytes)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisories (id, source_system, raw_source_id, vendor, advisory_type, severity, summary, evidence_tier, is_security, updated_at)
		VALUES
			('USN-7000-1', 'ubuntu_usn_api', '7000-1', 'ubuntu', 'security', 'critical', 'Kernel issue active only', 'vendor_db', true, now()),
			('USN-7000-2', 'ubuntu_usn_api', '7000-2', 'ubuntu', 'security', 'important', 'Kernel issue still in latest', 'vendor_db', true, now())
	`)
	require.NoError(t, err)

	_, err = backend.DB().Exec(ctx, `
		INSERT INTO advisory_references (id, advisory_id, ref_type, ref_value, url)
		VALUES
			($1, 'USN-7000-1', 'cve', 'CVE-2026-1111', 'https://example.com/CVE-2026-1111'),
			($2, 'USN-7000-2', 'cve', 'CVE-2026-2222', 'https://example.com/CVE-2026-2222')
	`, id.New("aref"), id.New("aref"))
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-7000-1",
		PackageName:    "linux-image-5.15.0-156-generic",
		InstalledNevra: utils.Some("linux-image-5.15.0-156-generic-0:5.15.0-156.166.amd64"),
		FixedNevra:     utils.Some("linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64"),
		Status:         "fixed_package_installed_pending_activation",
		Action:         "reboot_host",
		Severity:       utils.Some("critical"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "fixed_package_installed_kernel_not_running",
		ReasonText:     utils.Some("fixed kernel package is installed but the running kernel is older"),
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	err = queries.InsertDecisionRecord(ctx, db.InsertDecisionRecordParams{
		ID:             id.New("d"),
		HostID:         hostID,
		SnapshotID:     snapshotID,
		AdvisoryID:     "USN-7000-2",
		PackageName:    "linux-image-5.15.0-176-generic",
		InstalledNevra: utils.Some("linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64"),
		FixedNevra:     utils.Some("linux-image-5.15.0-178-generic-0:5.15.0-178.188.amd64"),
		Status:         "affected_fix_available",
		Action:         "update_package",
		Severity:       utils.Some("important"),
		EvidenceTier:   "vendor_db",
		ReasonCode:     "vendor_fix_available_not_installed",
		ReasonText:     utils.Some("a vendor fixed package is available but not installed"),
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	recorder := backend.HTTPGet(
		fmt.Sprintf("/api/v1/hosts/%s/kernel-posture", hostID),
		apitesting.WithBearerToken(adminToken),
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	assert.Equal(t, "5.15.0-156-generic", payload["running_kernel"])
	assert.Equal(t, "linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64", payload["latest_installed_kernel"])
	assert.Equal(t, true, payload["reboot_would_reduce_cve_count"])

	activeKernel := payload["active_kernel"].(map[string]any)
	assert.Equal(t, float64(2), activeKernel["advisory_count"])
	assert.Equal(t, float64(2), activeKernel["cve_count"])

	latestInstalled := payload["latest_installed"].(map[string]any)
	assert.Equal(t, float64(1), latestInstalled["advisory_count"])
	assert.Equal(t, float64(1), latestInstalled["cve_count"])

	notFoundPostureRecorder := backend.HTTPGet(
		"/api/v1/hosts/non-existent-host-id/kernel-posture",
		apitesting.WithBearerToken(adminToken),
	)
	assert.Equal(t, http.StatusNotFound, notFoundPostureRecorder.Code)
}
