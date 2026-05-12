package config

import "time"

type API struct {
	ListenAddress     string        `mapstructure:"listen_address" yaml:"listen_address,omitempty"`
	Port              int           `mapstructure:"port" yaml:"port,omitempty"`
	JWTSecretKey      string        `mapstructure:"jwt_secret_key" yaml:"jwt_secret_key,omitempty"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
}

func (a *API) Validate() error {
	if a.JWTSecretKey == "" {
		return ErrMissingConfig("api.jwt_secret_key")
	}
	return nil
}

const (
	DefaultAPIListenAddress  = "0.0.0.0"
	DefaultAPIPort           = 5199
	DefaultReadTimeout       = 5 * time.Second
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultWriteTimeout      = 60 * time.Second
	DefaultShutdownTimeout   = 10 * time.Second
)

func init() {
	SetDefault("api.listen_address", DefaultAPIListenAddress)
	SetDefault("api.port", DefaultAPIPort)
	SetDefault("api.read_timeout", DefaultReadTimeout)
	SetDefault("api.read_header_timeout", DefaultReadHeaderTimeout)
	SetDefault("api.write_timeout", DefaultWriteTimeout)
	SetDefault("api.shutdown_timeout", DefaultShutdownTimeout)
}
