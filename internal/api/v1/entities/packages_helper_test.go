package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
