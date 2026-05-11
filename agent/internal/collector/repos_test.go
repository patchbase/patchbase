package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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