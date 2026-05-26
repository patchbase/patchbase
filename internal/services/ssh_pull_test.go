package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/services"
	apitesting "go.patchbase.net/server/internal/testing"
	"google.golang.org/protobuf/proto"
)

func TestCleanQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"   ", ""},
		{"\"hello\"", "hello"},
		{"'world'", "world"},
		{"\"hello'world\"", "hello'world"},
		{"'hello\"world'", "hello\"world"},
		{"  \"hello\"  ", "hello"},
		{"noquotes", "noquotes"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, apitesting.TestCleanQuote(tc.input))
		})
	}
}

func TestCountAptPackageUpdates(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, int32(0), apitesting.TestCountAptPackageUpdates(""))
	})

	t.Run("with header and upgrades", func(t *testing.T) {
		output := `Listing... Done
curl/jammy-security,jammy-updates 7.81.0-1ubuntu1.16 amd64 [upgradable from: 7.81.0-1ubuntu1.15]
git/jammy-security,jammy-updates 1:2.34.1-1ubuntu1.10 amd64 [upgradable from: 1:2.34.1-1ubuntu1.9]
`
		assert.Equal(t, int32(2), apitesting.TestCountAptPackageUpdates(output))
	})

	t.Run("with warnings and interactive junk", func(t *testing.T) {
		output := `WARNING: apt does not have a stable CLI interface. Use with caution in scripts.
Listing... Done
libssl3/jammy-security,jammy-updates 3.0.2-0ubuntu1.15 amd64 [upgradable from: 3.0.2-0ubuntu1.14]
`
		assert.Equal(t, int32(1), apitesting.TestCountAptPackageUpdates(output))
	})
}

func TestCountRpmPackageUpdates(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, int32(0), apitesting.TestCountRpmPackageUpdates(""))
	})

	t.Run("dnf check-update clean list", func(t *testing.T) {
		output := `
curl.x86_64                            7.61.1-22.el8                   updates
git.x86_64                             2.27.0-1.el8                    updates
`
		assert.Equal(t, int32(2), apitesting.TestCountRpmPackageUpdates(output))
	})

	t.Run("dnf with headers and obsoletes", func(t *testing.T) {
		output := `
Last metadata expiration check: 0:12:34 ago on Fri May 22 15:29:42 2026.
Obsoleting Packages
kernel.x86_64                          4.18.0-372.9.1.el8              updates
`
		assert.Equal(t, int32(1), apitesting.TestCountRpmPackageUpdates(output))
	})
}

func TestParseUpgradablePackagesAPT(t *testing.T) {
	output := `Listing... Done
curl/jammy-security,jammy-updates 7.81.0-1ubuntu1.16 amd64 [upgradable from: 7.81.0-1ubuntu1.15]
git/jammy-security,jammy-updates 1:2.34.1-1ubuntu1.10 amd64 [upgradable from: 1:2.34.1-1ubuntu1.9]
`

	pkgs := apitesting.TestParseUpgradablePackages("apt", output)
	require.Len(t, pkgs, 2)
	assert.Equal(t, "curl", pkgs[0].GetName())
	assert.Equal(t, "7.81.0-1ubuntu1.16", pkgs[0].GetVersion())
	assert.Equal(t, "amd64", pkgs[0].GetArch())
	assert.Equal(t, "jammy-security,jammy-updates", pkgs[0].GetRepoOrigin())
	assert.Equal(t, "git", pkgs[1].GetName())
}

func TestParseUpgradablePackagesRPM(t *testing.T) {
	output := `
Last metadata expiration check: 0:12:34 ago on Fri May 22 15:29:42 2026.
curl.x86_64                            7.61.1-22.el8                   updates
openssl-libs.x86_64                    1:1.1.1k-14.el8_6               baseos
`

	pkgs := apitesting.TestParseUpgradablePackages("rpm", output)
	require.Len(t, pkgs, 2)
	assert.Equal(t, "curl", pkgs[0].GetName())
	assert.Equal(t, int32(0), pkgs[0].GetEpoch())
	assert.Equal(t, "7.61.1", pkgs[0].GetVersion())
	assert.Equal(t, "22.el8", pkgs[0].GetRelease())
	assert.Equal(t, "updates", pkgs[0].GetRepoOrigin())
	assert.Equal(t, "openssl-libs", pkgs[1].GetName())
	assert.Equal(t, int32(1), pkgs[1].GetEpoch())
	assert.Equal(t, "1.1.1k", pkgs[1].GetVersion())
}

func TestParseSSHPullReportAPTIncludesSourcePackage(t *testing.T) {
	output := `_PB_METADATA_HOSTNAME=apt-host
_PB_METADATA_ARCH=x86_64
_PB_METADATA_KERNEL=6.8.0-63-generic
_PB_METADATA_MACHINE_ID=machine-123
_PB_METADATA_IP=10.0.0.10
_PB_METADATA_BOOT_TIME=1716888000
_PB_METADATA_OS_ID=ubuntu
_PB_METADATA_OS_ID_LIKE=debian
_PB_METADATA_OS_NAME=Ubuntu
_PB_METADATA_OS_VERSION=24.04
---UPDATES_START---
Listing... Done
bash/noble-updates 5.2.21-2ubuntu4 amd64 [upgradable from: 5.2.21-2ubuntu3]
---PACKAGES_START---
bash|5.2.21-2ubuntu4|amd64|Ubuntu Developers|bash
linux-image-6.8.0-63-generic|6.8.0-63.63|amd64|Ubuntu Developers|linux
---REPOS_START---
deb http://archive.ubuntu.com/ubuntu noble main
`

	parsed, err := services.ParseSSHPullReport([]byte(output), time.Unix(1716888600, 0).UTC())
	require.NoError(t, err)
	require.Equal(t, "apt", parsed.OSFamily)
	require.Equal(t, int32(1), parsed.AvailableUpdates)

	var snapshot agentpb.AgentSnapshot
	require.NoError(t, proto.Unmarshal(parsed.Payload, &snapshot))
	require.Len(t, snapshot.GetPackages(), 2)
	assert.Equal(t, "bash", snapshot.GetPackages()[0].GetSourceRpm())
	assert.Equal(t, "linux", snapshot.GetPackages()[1].GetSourceRpm())
}

func TestParseSSHPullReportDetectsFamilyFromOSIDLike(t *testing.T) {
	output := `_PB_METADATA_HOSTNAME=mint-host
_PB_METADATA_ARCH=x86_64
_PB_METADATA_KERNEL=6.8.0
_PB_METADATA_MACHINE_ID=machine-999
_PB_METADATA_IP=10.0.0.20
_PB_METADATA_BOOT_TIME=1716888000
_PB_METADATA_OS_ID=unknownmint
_PB_METADATA_OS_ID_LIKE=ubuntu debian
_PB_METADATA_OS_NAME=Linux Mint
_PB_METADATA_OS_VERSION=22
---UPDATES_START---
Listing... Done
---PACKAGES_START---
base-files|12ubuntu4|amd64|Ubuntu Developers|base-files
---REPOS_START---
deb http://archive.ubuntu.com/ubuntu jammy main
`

	parsed, err := services.ParseSSHPullReport([]byte(output), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, "apt", parsed.OSFamily)
}
