package config

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

type DefaultValue struct {
	Key   string
	Value interface{}
}

var (
	Defaults = []DefaultValue{}
)

func New() (*Config, error) {
	return newConfig(Defaults)
}

func NewWithInjector(_ do.Injector) (*Config, error) {
	return newConfig(Defaults)
}

func NewWithSkipValidation() (*Config, error) {
	defaultValues := append(Defaults, DefaultValue{Key: "skip_validation", Value: true})
	return newConfig(defaultValues)
}

func NewWithDefaults(defaults []DefaultValue) (*Config, error) {
	return newConfig(slices.Concat(Defaults, defaults))
}

func newConfig(defaults []DefaultValue) (*Config, error) {
	logger := slog.Default().With("source", "config.New")

	v := viper.New()
	// Set config name (without extension)
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	configPath := os.Getenv("PATCHBASE_CONFIG")
	if configPath != "" {
		v.SetConfigFile(configPath)
	}

	// Add multiple search paths
	v.AddConfigPath(ConfigDir)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv() // Automatically bind env vars

	for _, def := range defaults {
		v.SetDefault(def.Key, def.Value)
		v.BindEnv(def.Key) // nolint: errcheck
	}

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("Config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		logger.With("config_file", v.ConfigFileUsed()).Debug("Config file loaded")
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	if !config.SkipValidation {
		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}
	}

	return &config, nil
}

func SetDefault(key string, value interface{}) {
	Defaults = append(Defaults, DefaultValue{
		Key:   key,
		Value: value,
	})
}

func ErrMissingConfig(key string) error {
	return fmt.Errorf("missing config: %s", key)
}

func ErrInvalidConfig(key string, reason string) error {
	return fmt.Errorf("invalid config: %s (%s)", key, reason)
}

const (
	ConfigDir = "/etc/patchbase-server"
)
