package collector

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agent "go.patchbase.net/proto/agent"
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

func TestParseAptPackageLine(t *testing.T) {
	line := "bash|5.2.21-2ubuntu4|amd64|Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>|bash"
	pkg, err := parseAptPackageLine(line)
	require.NoError(t, err)
	assert.Equal(t, "bash", pkg.Name)
	assert.Equal(t, int32(0), pkg.Epoch)
	assert.Equal(t, "5.2.21", pkg.Version)
	assert.Equal(t, "2ubuntu4", pkg.Release)
	assert.Equal(t, "amd64", pkg.Arch)
	assert.Equal(t, "bash-0:5.2.21-2ubuntu4.amd64", pkg.Nevra)
	assert.Equal(t, "bash", pkg.SourceRpm)
}

func TestParseAptPackageLineWithEpoch(t *testing.T) {
	line := "systemd|255.4-1ubuntu8.8|amd64|Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>|systemd"
	pkg, err := parseAptPackageLine(line)
	require.NoError(t, err)
	assert.Equal(t, int32(0), pkg.Epoch)

	line = "libfreetype6|2:2.13.2+dfsg-1build3|amd64|Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>|freetype"
	pkg, err = parseAptPackageLine(line)
	require.NoError(t, err)
	assert.Equal(t, int32(2), pkg.Epoch)
	assert.Equal(t, "2.13.2+dfsg", pkg.Version)
	assert.Equal(t, "1build3", pkg.Release)
	assert.Equal(t, "freetype", pkg.SourceRpm)
}

func TestParseAptPackageLineInvalid(t *testing.T) {
	_, err := parseAptPackageLine("invalid")
	assert.Error(t, err)
}

func TestCountAptPackageUpdates(t *testing.T) {
	output := `Listing... Done
bash/noble-updates 5.2.21-2ubuntu4 amd64 [upgradable from: 5.2.21-2ubuntu3]
linux-image-generic/noble-updates 6.8.0.40.40 amd64 [upgradable from: 6.8.0.35.35]
N: There is 1 additional version. Please use the '-a' switch to see it.
`
	count := countAptPackageUpdates(output)
	assert.Equal(t, int32(2), count)
}

func TestCollectInstalledPackagesAPT(t *testing.T) {
	runner := staticRunner{
		resultByCommand: map[string][]byte{
			"dpkg-query|-W|-f=${Package}|${Version}|${Architecture}|${Maintainer}|${source:Package}\\n": []byte(
				"bash|5.2.21-2ubuntu4|amd64|Ubuntu Developers|bash\n",
			),
		},
	}

	pkgs, err := CollectInstalledPackages(context.Background(), runner, agent.OsFamily_OS_FAMILY_APT)
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "bash", pkgs[0].Name)
	assert.Equal(t, "bash", pkgs[0].SourceRpm)
}

func TestCollectInstalledPackagesRPM(t *testing.T) {
	runner := staticRunner{
		resultByCommand: map[string][]byte{
			"rpm|-qa|--queryformat|%{NAME}|%{EPOCHNUM}|%{VERSION}|%{RELEASE}|%{ARCH}|%{SOURCERPM}|%{VENDOR}\\n": []byte(
				"bash|0|5.1.8|9.el9|x86_64|bash-5.1.8-9.el9.src.rpm|Rocky\n",
			),
		},
	}

	pkgs, err := CollectInstalledPackages(context.Background(), runner, agent.OsFamily_OS_FAMILY_RPM)
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "bash", pkgs[0].Name)
}

func TestCollectAvailablePackageUpdateCountAPT(t *testing.T) {
	runner := staticRunner{
		resultByCommand: map[string][]byte{
			"apt|list|--upgradable": []byte("Listing... Done\nbash/noble-updates 5.2 amd64 [upgradable from: 5.1]\n"),
		},
	}

	count, err := CollectAvailablePackageUpdateCount(context.Background(), runner, agent.OsFamily_OS_FAMILY_APT)
	require.NoError(t, err)
	assert.Equal(t, int32(1), count)
}

func TestCollectAvailablePackageUpdateCountIgnoresErrors(t *testing.T) {
	runner := staticRunner{
		errByCommand: map[string]error{
			"apt|list|--upgradable": errors.New("apt unavailable"),
		},
	}

	count, err := CollectAvailablePackageUpdateCount(context.Background(), runner, agent.OsFamily_OS_FAMILY_APT)
	require.NoError(t, err)
	assert.Equal(t, int32(0), count)
}

func TestCollectUpgradablePackagesAPT(t *testing.T) {
	runner := staticRunner{
		resultByCommand: map[string][]byte{
			"apt|list|--upgradable": []byte("Listing... Done\nbash/noble-updates 5.2.21-2ubuntu4 amd64 [upgradable from: 5.2.21-2ubuntu3]\n"),
		},
	}

	pkgs, err := CollectUpgradablePackages(context.Background(), runner, agent.OsFamily_OS_FAMILY_APT)
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "bash", pkgs[0].Name)
	assert.Equal(t, "noble-updates", pkgs[0].RepoOrigin)
	assert.Equal(t, "bash-0:5.2.21-2ubuntu4.amd64", pkgs[0].Nevra)
}

func TestCollectUpgradablePackagesRPM(t *testing.T) {
	runner := staticRunner{
		resultByCommand: map[string][]byte{
			"dnf|-q|--cacheonly|check-update": []byte("curl.x86_64 7.61.1-22.el8 updates\nopenssl-libs.x86_64 1:1.1.1k-14.el8_6 baseos\n"),
		},
	}

	pkgs, err := CollectUpgradablePackages(context.Background(), runner, agent.OsFamily_OS_FAMILY_RPM)
	require.NoError(t, err)
	require.Len(t, pkgs, 2)
	assert.Equal(t, "curl", pkgs[0].Name)
	assert.Equal(t, int32(0), pkgs[0].Epoch)
	assert.Equal(t, "7.61.1", pkgs[0].Version)
	assert.Equal(t, "22.el8", pkgs[0].Release)
	assert.Equal(t, "updates", pkgs[0].RepoOrigin)
	assert.Equal(t, "openssl-libs", pkgs[1].Name)
	assert.Equal(t, int32(1), pkgs[1].Epoch)
}

func TestCollectUpgradablePackagesRPMWithExitCode100(t *testing.T) {
	runner := staticRunner{
		outputWithErr: map[string][]byte{
			"dnf|-q|--cacheonly|check-update": []byte("curl.x86_64 7.61.1-22.el8 updates\nopenssl-libs.x86_64 1:1.1.1k-14.el8_6 baseos\n"),
		},
	}

	pkgs, err := CollectUpgradablePackages(context.Background(), runner, agent.OsFamily_OS_FAMILY_RPM)
	require.NoError(t, err)
	require.Len(t, pkgs, 2)
	assert.Equal(t, "curl", pkgs[0].Name)
	assert.Equal(t, "openssl-libs", pkgs[1].Name)
}

func TestCollectAvailablePackageUpdateCountRPMWithExitCode100(t *testing.T) {
	runner := staticRunner{
		outputWithErr: map[string][]byte{
			"dnf|-q|--cacheonly|check-update": []byte("curl.x86_64 7.61.1-22.el8 updates\nopenssl-libs.x86_64 1:1.1.1k-14.el8_6 baseos\n"),
		},
	}

	count, err := CollectAvailablePackageUpdateCount(context.Background(), runner, agent.OsFamily_OS_FAMILY_RPM)
	require.NoError(t, err)
	assert.Equal(t, int32(2), count)
}

type staticRunner struct {
	resultByCommand map[string][]byte
	errByCommand    map[string]error
	outputWithErr   map[string][]byte
}

func (r staticRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name
	for _, arg := range args {
		key += "|" + strings.ReplaceAll(arg, "\n", "\\n")
	}

	if output, ok := r.outputWithErr[key]; ok {
		return output, errors.New("exit status 100")
	}

	if err := r.errByCommand[key]; err != nil {
		return nil, err
	}

	if output, ok := r.resultByCommand[key]; ok {
		return output, nil
	}

	return nil, errors.New("command not mocked: " + key)
}
