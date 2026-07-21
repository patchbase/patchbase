// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"os"
	"path/filepath"
)

type SSL struct {
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled,omitempty"`
	CertificateFile string `mapstructure:"certificate_file" yaml:"certificate_file,omitempty"`
	KeyFile         string `mapstructure:"key_file" yaml:"key_file,omitempty"`
}

func (a *SSL) Validate() error {
	if !a.Enabled {
		return nil
	}
	if a.CertificateFile == "" {
		return ErrMissingConfig("ssl.certificate_file")
	}
	if a.KeyFile == "" {
		return ErrMissingConfig("ssl.key_file")
	}
	if stat, err := os.Stat(a.CertificateFile); err != nil || stat.IsDir() {
		return ErrInvalidConfig("ssl.certificate_file", "file does not exist or is a directory")
	}
	if stat, err := os.Stat(a.KeyFile); err != nil || stat.IsDir() {
		return ErrInvalidConfig("ssl.key_file", "file does not exist or is a directory")
	}
	return nil
}

var (
	DefaultSSLEnabled         = false
	DefaultSSLCertificateFile = filepath.Join(ConfigDir, "cert.pem")
	DefaultSSLKeyFile         = filepath.Join(ConfigDir, "key.pem")
)

func init() {
	SetDefault("ssl.enabled", DefaultSSLEnabled)
	SetDefault("ssl.certificate_file", DefaultSSLCertificateFile)
	SetDefault("ssl.key_file", DefaultSSLKeyFile)
}
