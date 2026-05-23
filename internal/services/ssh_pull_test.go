package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apitesting "go.patchbase.net/server/internal/testing"
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
