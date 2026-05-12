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
