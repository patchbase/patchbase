package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

func TestMapDecisionRow_CVEs(t *testing.T) {
	tests := []struct {
		name       string
		cvesInput  interface{}
		expectCVEs []CVEInfo
	}{
		{
			name:      "Valid JSON with IDs and URLs",
			cvesInput: []byte(`[{"id":"CVE-2023-1234","url":"https://nvd.nist.gov/vuln/detail/CVE-2023-1234"},{"id":"CVE-2023-5678","url":"https://cve.com/a,b|c"}]`),
			expectCVEs: []CVEInfo{
				{ID: "CVE-2023-1234", URL: "https://nvd.nist.gov/vuln/detail/CVE-2023-1234"},
				{ID: "CVE-2023-5678", URL: "https://cve.com/a,b|c"},
			},
		},
		{
			name:       "Nil CVEs",
			cvesInput:  nil,
			expectCVEs: []CVEInfo{},
		},
		{
			name:       "Empty JSON array",
			cvesInput:  []byte(`[]`),
			expectCVEs: []CVEInfo{},
		},
		{
			name:       "Invalid JSON input",
			cvesInput:  []byte(`invalid-json`),
			expectCVEs: []CVEInfo{},
		},
		{
			name:      "String type input",
			cvesInput: `[{"id":"CVE-2023-9999","url":"http://url-with-comma,and|pipe"}]`,
			expectCVEs: []CVEInfo{
				{ID: "CVE-2023-9999", URL: "http://url-with-comma,and|pipe"},
			},
		},
		{
			name: "Slice of maps input (pgx style)",
			cvesInput: []interface{}{
				map[string]interface{}{"id": "CVE-2026-33416", "url": "https://url1"},
			},
			expectCVEs: []CVEInfo{
				{ID: "CVE-2026-33416", URL: "https://url1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := sql.ListDecisionPageRowsBySnapshotRow{
				AdvisoryID: "USN-1234-1",
				Severity:   utils.None[string](),
				Cves:       tt.cvesInput,
			}

			item := MapDecisionRow(row, map[string]string{})
			assert.Equal(t, tt.expectCVEs, item.CVEs)
		})
	}
}

func TestGroupDecisionsByRemediation_CVEPropagation(t *testing.T) {
	cves := []CVEInfo{
		{ID: "CVE-2023-1234", URL: "https://url1"},
		{ID: "CVE-2023-5678", URL: "https://url2"},
	}

	decisions := []DecisionItem{
		{
			AdvisoryID:  "RLSA-2023-01",
			Title:       "Advisory Title",
			FamilyLabel: "glibc",
			PackageName: "glibc",
			CVEs:        cves,
		},
	}

	groups := GroupDecisionsByRemediation(decisions)
	assert.Len(t, groups, 1)
	assert.Len(t, groups[0].Advisories, 1)
	assert.Equal(t, cves, groups[0].Advisories[0].CVEs)
}

func TestDisplayPackageBuild(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		input       string
		expected    string
	}{
		{
			name:        "RPM x86_64",
			packageName: "openssl",
			input:       "openssl-0:1.1.1k-1.el8.x86_64",
			expected:    "1.1.1k-1.el8",
		},
		{
			name:        "Debian amd64",
			packageName: "linux-image-5.15.0-176-generic",
			input:       "linux-image-5.15.0-176-generic-0:5.15.0-176.186.amd64",
			expected:    "5.15.0-176.186",
		},
		{
			name:        "Debian binary suffix",
			packageName: "linux-image-5.15.0-176-generic",
			input:       "linux-image-5.15.0-176-generic-0:5.15.0-176.186.binary",
			expected:    "5.15.0-176.186",
		},
		{
			name:        "Debian with non-zero epoch and amd64",
			packageName: "libpostproc55",
			input:       "libpostproc55-7:4.4.2-0ubuntu0.22.04.1.amd64",
			expected:    "7:4.4.2-0ubuntu0.22.04.1",
		},
		{
			name:        "Debian with non-zero epoch and binary",
			packageName: "libpostproc55",
			input:       "libpostproc55-7:4.4.2-0ubuntu0.22.04.1.binary",
			expected:    "7:4.4.2-0ubuntu0.22.04.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayPackageBuild(tt.packageName, tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPackageFamilyLabel(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		sourceRPM   string
		expected    string
	}{
		{
			name:        "RPM with sourceRPM",
			packageName: "openssl-libs",
			sourceRPM:   "openssl-1.1.1k-1.el8.src.rpm",
			expected:    "openssl",
		},
		{
			name:        "RPM without sourceRPM fallback",
			packageName: "openssl-libs",
			sourceRPM:   "",
			expected:    "openssl",
		},
		{
			name:        "Debian kernel image package",
			packageName: "linux-image-5.15.0-176-generic",
			sourceRPM:   "",
			expected:    "kernel",
		},
		{
			name:        "Debian kernel headers package",
			packageName: "linux-headers-5.15.0-176-generic",
			sourceRPM:   "",
			expected:    "kernel",
		},
		{
			name:        "Debian kernel modules package",
			packageName: "linux-modules-extra-5.15.0-176-generic",
			sourceRPM:   "",
			expected:    "kernel",
		},
		{
			name:        "Debian kernel unsigned package",
			packageName: "linux-image-unsigned-5.15.0-176-generic",
			sourceRPM:   "",
			expected:    "kernel",
		},
		{
			name:        "Debian generic meta-package",
			packageName: "linux-image-generic",
			sourceRPM:   "",
			expected:    "kernel",
		},
		{
			name:        "Debian libc dev package",
			packageName: "linux-libc-dev",
			sourceRPM:   "",
			expected:    "kernel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packageFamilyLabel(tt.packageName, tt.sourceRPM)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGroupDecisionsByRemediation_SortsDeterministically(t *testing.T) {
	decisions := []DecisionItem{
		{
			AdvisoryID:        "USN-2",
			FamilyLabel:       "kernel",
			PackageName:       "linux-image-5.15.0-156-generic",
			InstalledNevra:    "5.15.0-156.166",
			SeverityLabel:     "Critical",
			SeverityTone:      "danger",
			ActionLabel:       "Reboot required",
			ActionTone:        "danger",
			AdvisoryUpdatedAt: "2026-05-21T10:00:00.000000Z",
		},
		{
			AdvisoryID:        "USN-1",
			FamilyLabel:       "openssl",
			PackageName:       "openssl-libs",
			InstalledNevra:    "3.0.7-1",
			SeverityLabel:     "Important",
			SeverityTone:      "warn",
			ActionLabel:       "Update available",
			ActionTone:        "info",
			AdvisoryUpdatedAt: "2026-05-23T10:00:00.000000Z",
		},
		{
			AdvisoryID:        "USN-2",
			FamilyLabel:       "kernel",
			PackageName:       "linux-image-5.15.0-176-generic",
			InstalledNevra:    "5.15.0-176.186",
			SeverityLabel:     "Critical",
			SeverityTone:      "danger",
			ActionLabel:       "Reboot required",
			ActionTone:        "danger",
			AdvisoryUpdatedAt: "2026-05-20T10:00:00.000000Z",
		},
	}

	groups := GroupDecisionsByRemediation(decisions)
	require.Len(t, groups, 2)

	assert.Equal(t, "kernel", groups[0].FamilyLabel)
	assert.Equal(t, "openssl", groups[1].FamilyLabel)

	require.Len(t, groups[0].Advisories, 1)
	require.Len(t, groups[0].Advisories[0].Items, 2)
	assert.Equal(t, "linux-image-5.15.0-156-generic", groups[0].Advisories[0].Items[0].PackageName)
	assert.Equal(t, "linux-image-5.15.0-176-generic", groups[0].Advisories[0].Items[1].PackageName)
}

func TestSeverityNormalization(t *testing.T) {
	assert.Equal(t, "Important", severityLabel("high"))
	assert.Equal(t, "warn", severityTone("high"))
	assert.Equal(t, "Moderate", severityLabel("medium"))
	assert.Equal(t, "info", severityTone("medium"))
}
