// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package collector

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOSRelease(t *testing.T) {
	input := `NAME="Rocky Linux"
ID="rocky"
VERSION_ID="9.5"
`

	result, err := ParseOsRelease(input)
	require.NoError(t, err)
	assert.Equal(t, "rocky", result.ID)
	assert.Equal(t, "Rocky Linux", result.Name)
	assert.Equal(t, "9.5", result.VersionID)
}

func TestParseOSReleaseMissingRequired(t *testing.T) {
	input := `NAME="Rocky Linux"
ID="rocky"
`

	_, err := ParseOsRelease(input)
	assert.Error(t, err)
}

func TestNormalizeOsFamily(t *testing.T) {
	tests := []struct {
		input    string
		expected int32
		wantErr  bool
	}{
		{"rhel", 1, false},
		{"rocky", 1, false},
		{"almalinux", 1, false},
		{"centos", 1, false},
		{"ol", 1, false},
		{"debian", 2, false},
		{"ubuntu", 2, false},
		{"RHEL", 1, false},
		{"Rocky", 1, false},
		{"suse", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := NormalizeOsFamily(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, int32(result))
			}
		})
	}
}

func TestParseMajorVersion(t *testing.T) {
	result, err := ParseMajorVersion("9.5")
	require.NoError(t, err)
	assert.Equal(t, int32(9), result)

	result, err = ParseMajorVersion("10")
	require.NoError(t, err)
	assert.Equal(t, int32(10), result)
}

func TestDetectArchitecture(t *testing.T) {
	tests := []struct {
		input    string
		expected int32
		wantErr  bool
	}{
		{"x86_64", 1, false},
		{"aarch64", 2, false},
		{"riscv64", 3, false},
		{"unknown", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := DetectArchitecture(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, int32(result))
			}
		})
	}
}

func TestReadMachineID(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/etc", 0755))
	require.NoError(t, afero.WriteFile(fs, "/etc/machine-id", []byte("abc123def456\n"), 0644))

	result, err := ReadMachineID(fs)
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", result)
}

func TestReadMachineIDFallback(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/var/lib/dbus", 0755))
	require.NoError(t, afero.WriteFile(fs, "/var/lib/dbus/machine-id", []byte("fallback-id\n"), 0644))

	result, err := ReadMachineID(fs)
	require.NoError(t, err)
	assert.Equal(t, "fallback-id", result)
}

func TestReadMachineIDNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	_, err := ReadMachineID(fs)
	assert.Error(t, err)
}

func TestReadUptime(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/proc/uptime", []byte("123456.78 234567.89\n"), 0644))

	result, err := ReadUptime(fs)
	require.NoError(t, err)
	assert.Equal(t, int64(123456), result)
}

func TestStripQuotes(t *testing.T) {
	assert.Equal(t, "hello", stripQuotes(`"hello"`))
	assert.Equal(t, "hello", stripQuotes(`'hello'`))
	assert.Equal(t, "hello", stripQuotes("hello"))
}
