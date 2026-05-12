package config

import "fmt"

type Config struct {
	SkipValidation bool     `mapstructure:"skip_validation" yaml:"skip_validation,omitempty"`
	API            API      `mapstructure:"api" yaml:"api,omitempty"`
	SSL            SSL      `mapstructure:"ssl" yaml:"ssl,omitempty"`
	Database       Database `mapstructure:"database" yaml:"database,omitempty"`
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
	return nil
}
