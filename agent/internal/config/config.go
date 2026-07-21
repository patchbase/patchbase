// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

const DefaultPath = "/etc/patchbase-agent/config.json"

type File struct {
	ServerURL         string `json:"server_url"`
	HostToken         string `json:"host_token"`
	CACert            string `json:"ca_cert,omitempty"`
	AllowInsecureHTTP bool   `json:"allow_insecure_http,omitempty"`
}

func DefaultFS() afero.Fs {
	return afero.NewOsFs()
}

func Save(fs afero.Fs, path string, cfg File) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := fs.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := afero.WriteFile(fs, path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func Load(fs afero.Fs, path string) (File, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, fmt.Errorf("config not found at %s: %w", path, err)
		}
		return File{}, fmt.Errorf("read config: %w", err)
	}

	var cfg File
	if err := json.Unmarshal(data, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}