package matchers

import (
	"testing"
	"time"

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
			evrStr:   "0:1.1.1",
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
	tests := []struct {
		name     string
		host     sql.Host
		repos    []*agentpb.Repo
		streams  []sql.ProductStream
		expected []string
	}{
		{
			name: "Rocky Linux matching baseos",
			host: sql.Host{
				OsFamily:     "rpm",
				OsName:       "Rocky Linux",
				OsMajor:      9,
				Architecture: "x86_64",
			},
			repos: []*agentpb.Repo{
				{
					RepoId:  "rocky-9-baseos",
					Enabled: true,
				},
			},
			streams: []sql.ProductStream{
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
			},
			expected: []string{"rocky:9-baseos"},
		},
		{
			name: "Ubuntu noble matching stream",
			host: sql.Host{
				OsFamily:     "apt",
				OsName:       "Ubuntu",
				OsMajor:      24,
				Architecture: "x86_64",
			},
			repos: []*agentpb.Repo{
				{
					RepoId:    "ssh_pull:1",
					RepoLabel: "noble main",
					Enabled:   true,
				},
			},
			streams: []sql.ProductStream{
				{
					ID:           "ubuntu:24-main",
					Vendor:       "ubuntu",
					DistroFamily: "ubuntu",
					DistroName:   "Ubuntu",
					MajorVersion: 24,
					Architecture: utils.Some("source"),
					RepoFamily:   "noble",
				},
			},
			expected: []string{"ubuntu:24-main"},
		},
		{
			name: "Debian bookworm matching stream",
			host: sql.Host{
				OsFamily:     "apt",
				OsName:       "Debian GNU/Linux",
				OsMajor:      12,
				Architecture: "x86_64",
			},
			repos: []*agentpb.Repo{
				{
					RepoId:    "ssh_pull:2",
					RepoLabel: "bookworm main",
					Enabled:   true,
				},
			},
			streams: []sql.ProductStream{
				{
					ID:           "debian:12-main",
					Vendor:       "debian",
					DistroFamily: "debian",
					DistroName:   "Debian GNU/Linux",
					MajorVersion: 12,
					Architecture: utils.Some("source"),
					RepoFamily:   "bookworm",
				},
			},
			expected: []string{"debian:12-main"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved := resolveProductStreams(tc.host, tc.repos, tc.streams)
			got := make([]string, 0, len(resolved))
			for _, r := range resolved {
				got = append(got, r.ID)
			}
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestPackageMatchKeys(t *testing.T) {
	t.Run("includes package and source package", func(t *testing.T) {
		pkg := &agentpb.Package{
			Name:      "linux-image-6.8.0-117-generic",
			SourceRpm: "linux",
		}
		assert.Equal(t, []string{"linux-image-6.8.0-117-generic", "linux"}, packageMatchKeys(pkg))
	})

	t.Run("deduplicates identical package and source", func(t *testing.T) {
		pkg := &agentpb.Package{
			Name:      "openssl",
			SourceRpm: "openssl",
		}
		assert.Equal(t, []string{"openssl"}, packageMatchKeys(pkg))
	})
}

func TestCandidatePackagesDeduplicatesAcrossKeys(t *testing.T) {
	pkg := &agentpb.Package{
		Name:      "linux-image-6.8.0-117-generic",
		SourceRpm: "linux",
		Nevra:     "linux-image-6.8.0-117-generic-0:6.8.0-117.117.amd64",
	}
	packagesByKey := indexPackagesByName([]*agentpb.Package{pkg})

	rulesByPackage := map[string][]sql.AffectedPackageRule{
		"linux-image-6.8.0-117-generic": {
			{ID: "rule-binary"},
		},
		"linux": {
			{ID: "rule-source"},
		},
	}

	candidates := candidatePackages(packagesByKey, rulesByPackage, map[string][]sql.FixedPackage{})
	require.Len(t, candidates, 1)
	assert.Equal(t, pkg.GetName(), candidates[0].GetName())
}

func TestMatchesPackageArch(t *testing.T) {
	tests := []struct {
		name      string
		pkgArch   string
		ruleArch  string
		osFamily  string
		shouldHit bool
	}{
		{
			name:      "apt binary wildcard",
			pkgArch:   "amd64",
			ruleArch:  "binary",
			osFamily:  "apt",
			shouldHit: true,
		},
		{
			name:      "apt source wildcard",
			pkgArch:   "amd64",
			ruleArch:  "source",
			osFamily:  "apt",
			shouldHit: true,
		},
		{
			name:      "apt exact arch",
			pkgArch:   "amd64",
			ruleArch:  "amd64",
			osFamily:  "apt",
			shouldHit: true,
		},
		{
			name:      "apt mismatched concrete arch",
			pkgArch:   "amd64",
			ruleArch:  "arm64",
			osFamily:  "apt",
			shouldHit: false,
		},
		{
			name:      "rpm exact arch only",
			pkgArch:   "x86_64",
			ruleArch:  "source",
			osFamily:  "rpm",
			shouldHit: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.shouldHit, matchesPackageArch(tc.pkgArch, tc.ruleArch, tc.osFamily))
		})
	}
}

func TestParseRunningKernelEVR(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected evr
		wantErr  bool
	}{
		{
			name:  "standard with package name and arch",
			input: "kernel-6.12.0-124.55.3.el10_1.x86_64",
			expected: evr{
				epoch:   0,
				version: "6.12.0",
				release: "124.55.3.el10_1",
			},
			wantErr: false,
		},
		{
			name:  "without package name but with arch",
			input: "6.12.0-124.55.3.el10_1.x86_64",
			expected: evr{
				epoch:   0,
				version: "6.12.0",
				release: "124.55.3.el10_1",
			},
			wantErr: false,
		},
		{
			name:  "different release version without package name",
			input: "6.12.0-124.56.1.el10_1.x86_64",
			expected: evr{
				epoch:   0,
				version: "6.12.0",
				release: "124.56.1.el10_1",
			},
			wantErr: false,
		},
		{
			name:    "no release separator",
			input:   "6.12.0",
			wantErr: true,
		},
		{
			name:    "no dot at all",
			input:   "6-12",
			wantErr: true,
		},
		{
			name:    "completely malformed",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRunningKernelEVR(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestDebianEVR_ParseAndCompare(t *testing.T) {
	tests := []struct {
		name     string
		left     string
		right    string
		expected int // -1 left < right, 0 equal, 1 left > right
	}{
		{
			name:     "equal simple",
			left:     "1.0-1",
			right:    "1.0-1",
			expected: 0,
		},
		{
			name:     "epoch higher",
			left:     "1:1.0-1",
			right:    "2.0-1",
			expected: 1,
		},
		{
			name:     "version higher",
			left:     "2.0-1",
			right:    "1.9-1",
			expected: 1,
		},
		{
			name:     "release higher",
			left:     "1.0-2",
			right:    "1.0-1",
			expected: 1,
		},
		{
			name:     "tilde sorts before empty",
			left:     "1.0~beta1-1",
			right:    "1.0-1",
			expected: -1,
		},
		{
			name:     "plus sorts after empty",
			left:     "1.0+b1-1",
			right:    "1.0-1",
			expected: 1,
		},
		{
			name:     "letters sort before non-letters",
			left:     "1.0a-1",
			right:    "1.0+-1",
			expected: -1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, err := parseDebianEVR(tc.left)
			require.NoError(t, err)
			r, err := parseDebianEVR(tc.right)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, compareDebianEVR(l, r))
		})
	}
}

func TestDebianEVR_ParseFromNEVR(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected evr
	}{
		{
			name:  "standard with epoch, release, arch",
			input: "openssl-0:3.0.7-2.amd64",
			expected: evr{
				epoch:   0,
				version: "3.0.7",
				release: "2",
			},
		},
		{
			name:  "no epoch, with arch",
			input: "openssl-3.0.7-2.amd64",
			expected: evr{
				epoch:   0,
				version: "3.0.7",
				release: "2",
			},
		},
		{
			name:  "no release, with arch",
			input: "openssl-3.0.7.amd64",
			expected: evr{
				epoch:   0,
				version: "3.0.7",
				release: "",
			},
		},
		{
			name:  "no arch, with release",
			input: "linux-image-generic-6.8.0-31",
			expected: evr{
				epoch:   0,
				version: "6.8.0",
				release: "31",
			},
		},
		{
			name:  "no arch, release contains dot and letter suffix",
			input: "openssl-1.0-1.deb12u1",
			expected: evr{
				epoch:   0,
				version: "1.0",
				release: "1.deb12u1",
			},
		},
		{
			name:  "no arch, release contains dot and digit suffix",
			input: "openssl-3.0.2-0ubuntu1.1",
			expected: evr{
				epoch:   0,
				version: "3.0.2",
				release: "0ubuntu1.1",
			},
		},
		{
			name:  "no arch, release contains multiple dot-separated suffixes",
			input: "openssl-3.0.2-0ubuntu1.1.20230101",
			expected: evr{
				epoch:   0,
				version: "3.0.2",
				release: "0ubuntu1.1.20230101",
			},
		},
		{
			name:  "loong64 architecture suffix",
			input: "openssl-3.0.7-2.loong64",
			expected: evr{
				epoch:   0,
				version: "3.0.7",
				release: "2",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDebianEVRFromNEVR(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestDebianEVR_EvaluateRules(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		rule      string
		expected  bool
	}{
		{
			name:      "less than match",
			installed: "1.0-1",
			rule:      "< 2.0-1",
			expected:  true,
		},
		{
			name:      "less than no match",
			installed: "2.0-1",
			rule:      "< 1.0-1",
			expected:  false,
		},
		{
			name:      "equal match",
			installed: "1.0-1",
			rule:      "= 1.0-1",
			expected:  true,
		},
		{
			name:      "greater than or equal match",
			installed: "2.0-1",
			rule:      ">= 2.0-1",
			expected:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inst, err := parseDebianEVR(tc.installed)
			require.NoError(t, err)
			ok, err := evaluateDebianEVRRule(inst, tc.rule)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestCollapseSupersededDecisions_APT(t *testing.T) {
	dec1 := decision{
		record: sql.InsertDecisionRecordParams{
			HostID:             "host1",
			SnapshotID:         "snap1",
			AdvisoryID:         "adv1",
			InstalledPackageID: utils.None[string](),
			PackageName:        "openssl",
			InstalledNevra:     utils.Some("openssl-1.0-1"),
			FixedNevra:         utils.Some("openssl-1.0-1.deb12u1"),
			Action:             "update_package",
		},
		severity: "critical",
	}
	dec2 := decision{
		record: sql.InsertDecisionRecordParams{
			HostID:             "host1",
			SnapshotID:         "snap1",
			AdvisoryID:         "adv2",
			InstalledPackageID: utils.None[string](),
			PackageName:        "openssl",
			InstalledNevra:     utils.Some("openssl-1.0-1"),
			FixedNevra:         utils.Some("openssl-1.0-1.deb12u2"), // newer!
			Action:             "update_package",
		},
		severity: "critical",
	}

	collapsed := collapseSupersededDecisions([]decision{dec1, dec2}, "apt")
	require.Len(t, collapsed, 1)
	assert.Equal(t, "openssl-1.0-1.deb12u2", collapsed[0].record.FixedNevra.UnwrapOr(""))
}

func TestLatestInstalledKernel(t *testing.T) {
	t.Run("APT packages", func(t *testing.T) {
		packages := []*agentpb.Package{
			{Name: "linux-image-5.15.0-120-generic", Version: "5.15.0", Release: "120.130"},
			{Name: "linux-image-5.15.0-176-generic", Version: "5.15.0", Release: "176.186"},
			{Name: "linux-image-5.15.0-156-generic", Version: "5.15.0", Release: "156.166"},
			{Name: "linux-image-generic", Version: "5.15.0.176.163", Release: "1"},
		}

		latest, found := latestInstalledKernelEVRAPT(packages, "generic")
		require.True(t, found)
		assert.Equal(t, int64(0), latest.epoch)
		assert.Equal(t, "5.15.0", latest.version)
		assert.Equal(t, "176.186", latest.release)
	})

	t.Run("RPM packages", func(t *testing.T) {
		packages := []*agentpb.Package{
			{Name: "kernel", Version: "5.14.0", Release: "70.el9"},
			{Name: "kernel", Version: "5.14.0", Release: "362.24.1.el9"},
			{Name: "kernel", Version: "5.14.0", Release: "284.11.1.el9"},
		}

		latest, found := latestInstalledKernelEVRRPM(packages, "kernel")
		require.True(t, found)
		assert.Equal(t, int64(0), latest.epoch)
		assert.Equal(t, "5.14.0", latest.version)
		assert.Equal(t, "362.24.1.el9", latest.release)
	})
}

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"high", "important"},
		{"medium", "moderate"},
		{"critical", "critical"},
		{"low", "low"},
		{"  HIGH  ", "important"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, normalizeSeverity(tc.input))
		})
	}
}

func TestSeverityPriority(t *testing.T) {
	assert.Equal(t, 4, severityPriority("critical"))
	assert.Equal(t, 3, severityPriority("important"))
	assert.Equal(t, 3, severityPriority("high"))
	assert.Equal(t, 2, severityPriority("moderate"))
	assert.Equal(t, 2, severityPriority("medium"))
	assert.Equal(t, 1, severityPriority("low"))
	assert.Equal(t, 0, severityPriority("unknown"))
}

func TestAggregateHostCurrentState(t *testing.T) {
	m := &matcher{}
	snapshot := sql.HostSnapshot{HostID: "h1", ID: "s1"}
	decisions := []decision{
		{
			record:   sql.InsertDecisionRecordParams{Status: "affected_fix_available", Action: "update_package"},
			severity: "high",
		},
		{
			record:   sql.InsertDecisionRecordParams{Status: "affected_fix_available", Action: "update_package"},
			severity: "medium",
		},
		{
			record:   sql.InsertDecisionRecordParams{Status: "affected_no_fix", Action: "investigate"},
			severity: "critical",
		},
		{
			record:   sql.InsertDecisionRecordParams{Status: "resolved", Action: "none"},
			severity: "critical",
		},
	}

	state := m.aggregateHostCurrentState(snapshot, decisions, time.Now(), 2)

	assert.Equal(t, int32(1), state.CriticalCount)
	assert.Equal(t, int32(1), state.ImportantCount)
	assert.Equal(t, int32(1), state.ModerateCount)
	assert.Equal(t, int32(2), state.ActionableCount)
	assert.Equal(t, int32(1), state.NoFix)
	assert.Equal(t, int32(2), state.AvailableUpdates)
}
