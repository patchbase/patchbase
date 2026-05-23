package matchers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

func TestEVR_ParseAndCompare(t *testing.T) {
	tests := []struct {
		name     string
		evrStr   string
		expected evr
		wantErr  bool
	}{
		{
			name:   "standard evr",
			evrStr: "0:1.1.1-1.el9",
			expected: evr{
				epoch:   0,
				version: "1.1.1",
				release: "1.el9",
			},
			wantErr: false,
		},
		{
			name:   "non-zero epoch",
			evrStr: "2:2.4.5-9",
			expected: evr{
				epoch:   2,
				version: "2.4.5",
				release: "9",
			},
			wantErr: false,
		},
		{
			name:     "missing release separator",
			evrStr:  "0:1.1.1",
			expected: evr{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEVR(tc.evrStr)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestEVR_Compare(t *testing.T) {
	tests := []struct {
		name     string
		left     string
		right    string
		expected int // -1 left < right, 0 equal, 1 left > right
	}{
		{
			name:     "equal simple",
			left:     "0:1.0-1",
			right:    "0:1.0-1",
			expected: 0,
		},
		{
			name:     "epoch higher",
			left:     "1:1.0-1",
			right:    "0:2.0-1",
			expected: 1,
		},
		{
			name:     "version higher",
			left:     "0:2.0-1",
			right:    "0:1.9-1",
			expected: 1,
		},
		{
			name:     "release higher",
			left:     "0:1.0-2",
			right:    "0:1.0-1",
			expected: 1,
		},
		{
			name:     "alphanumeric segment comparison",
			left:     "0:1.0a-1",
			right:    "0:1.0-1",
			expected: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, err := parseEVR(tc.left)
			require.NoError(t, err)
			r, err := parseEVR(tc.right)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, compareEVR(l, r))
		})
	}
}

func TestEVR_EvaluateRules(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		rule      string
		expected  bool
	}{
		{
			name:      "less than match",
			installed: "0:1.0-1",
			rule:      "< 0:2.0-1",
			expected:  true,
		},
		{
			name:      "less than no match",
			installed: "0:2.0-1",
			rule:      "< 0:1.0-1",
			expected:  false,
		},
		{
			name:      "equal match",
			installed: "0:1.0-1",
			rule:      "= 0:1.0-1",
			expected:  true,
		},
		{
			name:      "greater than or equal match",
			installed: "0:2.0-1",
			rule:      ">= 0:2.0-1",
			expected:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inst, err := parseEVR(tc.installed)
			require.NoError(t, err)
			ok, err := evaluateEVRRule(inst, tc.rule)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestResolveProductStreams(t *testing.T) {
	host := sql.Host{
		OsFamily:     "rpm",
		OsName:       "Rocky Linux",
		OsMajor:      9,
		Architecture: "x86_64",
	}

	repos := []*agentpb.Repo{
		{
			RepoId:  "rocky-9-baseos",
			Enabled: true,
		},
	}

	streams := []sql.ProductStream{
		{
			ID:           "rocky:9-baseos",
			Vendor:       "rocky",
			DistroFamily: "rpm",
			DistroName:   "Rocky Linux",
			MajorVersion: 9,
			Architecture: utils.Some("x86_64"),
			RepoFamily:   "baseos",
		},
		{
			ID:           "rocky:9-appstream",
			Vendor:       "rocky",
			DistroFamily: "rpm",
			DistroName:   "Rocky Linux",
			MajorVersion: 9,
			Architecture: utils.Some("x86_64"),
			RepoFamily:   "appstream",
		},
		{
			ID:           "alma:9",
			Vendor:       "alma",
			DistroFamily: "rpm",
			DistroName:   "AlmaLinux",
			MajorVersion: 9,
			Architecture: utils.Some("x86_64"),
			RepoFamily:   "all",
		},
	}

	resolved := resolveProductStreams(host, repos, streams)
	require.Len(t, resolved, 1)
	assert.Equal(t, "rocky:9-baseos", resolved[0].ID)
}
