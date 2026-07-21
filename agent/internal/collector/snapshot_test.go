// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package collector

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestReadOsReleaseWithFS(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/etc", 0755))
	require.NoError(t, afero.WriteFile(fs, "/etc/os-release", []byte(`NAME="Rocky Linux"
ID="rocky"
VERSION_ID="9.5"
`), 0644))

	result, err := ReadOsRelease(fs)
	require.NoError(t, err)
	if result.ID != "rocky" {
		t.Errorf("expected ID=rocky, got %s", result.ID)
	}
	if result.Name != "Rocky Linux" {
		t.Errorf("expected Name=Rocky Linux, got %s", result.Name)
	}
	if result.VersionID != "9.5" {
		t.Errorf("expected VersionID=9.5, got %s", result.VersionID)
	}
}