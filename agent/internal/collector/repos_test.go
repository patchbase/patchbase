// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package collector

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agent "go.patchbase.net/proto/agent"
)

func TestParseRepoFile(t *testing.T) {
	input := `[baseos]
name=Rocky Linux 9 - BaseOS
enabled=1
baseurl=https://dl.rockylinux.org/pub/rocky/9/BaseOS/x86_64/os/

[debug]
name=Disabled Repo
enabled=0
`
	repos := parseRepoFile(input)
	require.Len(t, repos, 1)
	assert.Equal(t, "baseos", repos[0].RepoId)
	assert.True(t, repos[0].Enabled)
	assert.Equal(t, "Rocky Linux 9 - BaseOS", repos[0].RepoLabel)
	assert.Equal(t, "https://dl.rockylinux.org/pub/rocky/9/BaseOS/x86_64/os/", repos[0].Baseurl)
}

func TestParseRepoFileDeduplicates(t *testing.T) {
	input := `[baseos]
name=Rocky Linux 9 - BaseOS
enabled=1

[baseos]
name=Rocky Linux 9 - BaseOS (duplicate)
enabled=1
`
	repos := parseRepoFile(input)
	require.Len(t, repos, 1)
}

func TestIsTruthy(t *testing.T) {
	assert.True(t, isTruthy("1"))
	assert.True(t, isTruthy("true"))
	assert.True(t, isTruthy("True"))
	assert.True(t, isTruthy("yes"))
	assert.True(t, isTruthy("YES"))
	assert.False(t, isTruthy("0"))
	assert.False(t, isTruthy("false"))
	assert.False(t, isTruthy("no"))
}

func TestFirstLiteralURL(t *testing.T) {
	assert.Equal(t, "https://example.com/repo", firstLiteralURL("https://example.com/repo"))
	assert.Equal(t, "", firstLiteralURL("$releasever"))
	assert.Equal(t, "", firstLiteralURL(""))
	assert.Equal(t, "https://example.com/repo", firstLiteralURL("https://example.com/repo http://other.com/repo"))
}

func TestCollectEnabledReposAPTList(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/etc/apt", 0755))
	require.NoError(t, afero.WriteFile(fs, "/etc/apt/sources.list", []byte(`
deb http://archive.ubuntu.com/ubuntu noble main restricted
deb-src http://archive.ubuntu.com/ubuntu noble main restricted
`), 0644))

	repos, err := CollectEnabledRepos(fs, agent.OsFamily_OS_FAMILY_APT)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, "/etc/apt/sources.list:2", repos[0].RepoId)
	assert.Equal(t, "noble main restricted", repos[0].RepoLabel)
	assert.Equal(t, "http://archive.ubuntu.com/ubuntu", repos[0].Baseurl)
}

func TestCollectEnabledReposAPTSources(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/etc/apt/sources.list.d", 0755))
	require.NoError(t, afero.WriteFile(fs, "/etc/apt/sources.list.d/custom.sources", []byte(`
Types: deb
URIs: https://packages.example.test/repo
Suites: noble noble-updates
Components: main universe
Enabled: yes
`), 0644))

	repos, err := CollectEnabledRepos(fs, agent.OsFamily_OS_FAMILY_APT)
	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, "https://packages.example.test/repo", repos[0].Baseurl)
	assert.Equal(t, "noble main universe", repos[0].RepoLabel)
	assert.Equal(t, "noble-updates main universe", repos[1].RepoLabel)
}

func TestParseAptListLineWithOptions(t *testing.T) {
	uri, suite, components, ok := parseAptListLine("deb [arch=amd64 signed-by=/etc/apt/keyrings/example.gpg] https://repo.example.test stable main")
	require.True(t, ok)
	assert.Equal(t, "https://repo.example.test", uri)
	assert.Equal(t, "stable", suite)
	assert.Equal(t, []string{"main"}, components)
}
