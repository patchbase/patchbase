// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package config_test

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.patchbase.net/agent/internal/config"
)

func TestSaveAndLoadConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.json"

	cfg := config.File{
		ServerURL:         "https://patchbase.local",
		HostToken:         "token123",
		CACert:            "/tmp/ca.pem",
		AllowInsecureHTTP: true,
	}

	err := config.Save(fs, path, cfg)
	require.NoError(t, err)

	loaded, err := config.Load(fs, path)
	require.NoError(t, err)
	assert.Equal(t, cfg.ServerURL, loaded.ServerURL)
	assert.Equal(t, cfg.HostToken, loaded.HostToken)
	assert.Equal(t, cfg.CACert, loaded.CACert)
	assert.Equal(t, cfg.AllowInsecureHTTP, loaded.AllowInsecureHTTP)
}

func TestLoadConfigNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	_, err := config.Load(fs, "/nonexistent/path")
	assert.Error(t, err)
}

func TestSaveConfigCreatesParentDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/etc/patchbase-agent/config.json"

	cfg := config.File{
		ServerURL: "https://patchbase.local",
		HostToken: "token123",
	}

	err := config.Save(fs, path, cfg)
	require.NoError(t, err)

	loaded, err := config.Load(fs, path)
	require.NoError(t, err)
	assert.Equal(t, "https://patchbase.local", loaded.ServerURL)
}