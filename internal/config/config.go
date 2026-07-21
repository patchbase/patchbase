// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"fmt"
	"time"
)

type Config struct {
	SkipValidation bool         `mapstructure:"skip_validation" yaml:"skip_validation,omitempty"`
	API            API          `mapstructure:"api" yaml:"api,omitempty"`
	SSL            SSL          `mapstructure:"ssl" yaml:"ssl,omitempty"`
	Database       Database     `mapstructure:"database" yaml:"database,omitempty"`
	SSH            SSH          `mapstructure:"ssh" yaml:"ssh,omitempty"`
	AdvisorySync   AdvisorySync `mapstructure:"advisory_sync" yaml:"advisory_sync,omitempty"`

	EncryptionKey string `mapstructure:"encryption_key" yaml:"encryption_key,omitempty"`
}

type AdvisorySync struct {
	BaseURL         string         `mapstructure:"base_url" yaml:"base_url,omitempty"`
	RefreshInterval time.Duration  `mapstructure:"refresh_interval" yaml:"refresh_interval,omitempty"`
	StorageDir      string         `mapstructure:"storage_dir" yaml:"storage_dir,omitempty"`
	ScopeMappings   []ScopeMapping `mapstructure:"scope_mappings" yaml:"scope_mappings,omitempty"`
}

type ScopeMapping struct {
	Match MatchRules `mapstructure:"match" yaml:"match"`
	Scope string     `mapstructure:"scope" yaml:"scope"`
}

type MatchRules struct {
	OSFamily     string `mapstructure:"os_family" yaml:"os_family,omitempty"`
	OSName       string `mapstructure:"os_name" yaml:"os_name,omitempty"`
	OSVersion    string `mapstructure:"os_version" yaml:"os_version,omitempty"`
	OSMajor      int32  `mapstructure:"os_major" yaml:"os_major,omitempty"`
	Architecture string `mapstructure:"architecture" yaml:"architecture,omitempty"`
}

func (a *AdvisorySync) Validate() error {
	if a.BaseURL == "" {
		return ErrMissingConfig("advisory_sync.base_url")
	}
	if a.RefreshInterval <= 0 {
		return ErrInvalidConfig("advisory_sync.refresh_interval", "must be greater than zero")
	}
	if a.StorageDir == "" {
		return ErrMissingConfig("advisory_sync.storage_dir")
	}
	for i, mapping := range a.ScopeMappings {
		if mapping.Scope == "" {
			return ErrMissingConfig(fmt.Sprintf("advisory_sync.scope_mappings[%d].scope", i))
		}
	}
	return nil
}

func (c *Config) Validate() error {
	if err := c.API.Validate(); err != nil {
		return fmt.Errorf("invalid api config: %w", err)
	}
	if err := c.SSL.Validate(); err != nil {
		return fmt.Errorf("invalid ssl config: %w", err)
	}
	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}
	if err := c.SSH.Validate(); err != nil {
		return fmt.Errorf("invalid ssh config: %w", err)
	}
	if err := c.AdvisorySync.Validate(); err != nil {
		return fmt.Errorf("invalid advisory_sync config: %w", err)
	}
	if c.EncryptionKey == "" {
		return ErrMissingConfig("encryption_key")
	}
	return nil
}

const (
	DefaultEncryptionKey               = ""
	DefaultAdvisorySyncBaseURL         = "https://dl.patchbase.net/v1/advisory-db"
	DefaultAdvisorySyncRefreshInterval = 6 * time.Hour
	DefaultAdvisorySyncStorageDir      = "/var/lib/patchbase-server/db/advisories"
)

func init() {
	SetDefault("encryption_key", DefaultEncryptionKey)
	SetDefault("advisory_sync.base_url", DefaultAdvisorySyncBaseURL)
	SetDefault("advisory_sync.refresh_interval", DefaultAdvisorySyncRefreshInterval)
	SetDefault("advisory_sync.storage_dir", DefaultAdvisorySyncStorageDir)
}
