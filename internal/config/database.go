// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package config

type Database struct {
	URL      string `mapstructure:"url" yaml:"url,omitempty"`
	LogLevel string `mapstructure:"log_level" yaml:"log_level,omitempty"`
}

func (d *Database) Validate() error {
	if d.URL == "" {
		return ErrMissingConfig("database.url")
	}
	return nil
}

var (
	DefaultDatabaseLogLevel = "error"
)

func init() {
	SetDefault("database.log_level", DefaultDatabaseLogLevel)
}
