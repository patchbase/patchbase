package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageLine(t *testing.T) {
	line := "bash|0|5.1.8|9.el9|x86_64|bash-5.1.8-9.el9.src.rpm|Rocky"
	pkg, err := parsePackageLine(line)
	require.NoError(t, err)
	assert.Equal(t, "bash", pkg.Name)
	assert.Equal(t, int32(0), pkg.Epoch)
	assert.Equal(t, "5.1.8", pkg.Version)
	assert.Equal(t, "9.el9", pkg.Release)
	assert.Equal(t, "x86_64", pkg.Arch)
	assert.Equal(t, "bash-0:5.1.8-9.el9.x86_64", pkg.Nevra)
	assert.Equal(t, "bash-5.1.8-9.el9.src.rpm", pkg.SourceRpm)
	assert.Equal(t, "Rocky", pkg.Vendor)
}

func TestParsePackageLineWithEpoch(t *testing.T) {
	line := "docker-ce|3|28.0.1|1.el10|x86_64|docker-ce-28.0.1-1.el10.src.rpm|Docker Inc."
	pkg, err := parsePackageLine(line)
	require.NoError(t, err)
	assert.Equal(t, "docker-ce", pkg.Name)
	assert.Equal(t, int32(3), pkg.Epoch)
	assert.Equal(t, "docker-ce-3:28.0.1-1.el10.x86_64", pkg.Nevra)
}

func TestParsePackageLineInvalid(t *testing.T) {
	_, err := parsePackageLine("invalid|line")
	assert.Error(t, err)
}

func TestParseEpoch(t *testing.T) {
	result, err := parseEpoch("0")
	require.NoError(t, err)
	assert.Equal(t, int32(0), result)

	result, err = parseEpoch("3")
	require.NoError(t, err)
	assert.Equal(t, int32(3), result)

	result, err = parseEpoch("(none)")
	require.NoError(t, err)
	assert.Equal(t, int32(0), result)

	result, err = parseEpoch("")
	require.NoError(t, err)
	assert.Equal(t, int32(0), result)
}

func TestCountPackageUpdates(t *testing.T) {
	output := `Last metadata expiration check: 0:12:34 ago on Mon 24 Mar 2026 10:00:00 AM UTC.

bash.x86_64                     5.2.26-1.el9                    baseos
kernel.x86_64                   5.14.0-503.40.1.el9_5           baseos
kernel-core.x86_64              5.14.0-503.40.1.el9_5           baseos

Obsoleting Packages
oldpkg.x86_64                   1.2.3-4.el9                     appstream
`
	count := countPackageUpdates(output)
	assert.Equal(t, int32(4), count)
}

func TestCountPackageUpdatesEmpty(t *testing.T) {
	output := `
Last metadata expiration check: 0:00:05 ago on Mon 24 Mar 2026 10:00:00 AM UTC.
`
	count := countPackageUpdates(output)
	assert.Equal(t, int32(0), count)
}