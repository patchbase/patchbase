package config

import "time"

type API struct {
	ListenAddress       string        `mapstructure:"listen_address" yaml:"listen_address,omitempty"`
	Port                int           `mapstructure:"port" yaml:"port,omitempty"`
	JWTSecretKey        string        `mapstructure:"jwt_secret_key" yaml:"jwt_secret_key,omitempty"`
	RequestLogLevel     string        `mapstructure:"request_log_level" yaml:"request_log_level,omitempty"`
	ReadTimeout         time.Duration `mapstructure:"read_timeout"`
	ReadHeaderTimeout   time.Duration `mapstructure:"read_header_timeout"`
	WriteTimeout        time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout     time.Duration `mapstructure:"shutdown_timeout"`
	MaxRequestBodyBytes int64         `mapstructure:"max_request_body_bytes" yaml:"max_request_body_bytes,omitempty"`
}

func (a *API) Validate() error {
	if a.JWTSecretKey == "" {
		return ErrMissingConfig("api.jwt_secret_key")
	}
	if a.RequestLogLevel == "" {
		return ErrMissingConfig("api.request_log_level")
	} else if a.RequestLogLevel != "debug" && a.RequestLogLevel != "info" && a.RequestLogLevel != "warn" && a.RequestLogLevel != "error" {
		return ErrInvalidConfig("api.request_log_level", "must be one of: debug, info, warn, error")
	}
	return nil
}

const (
	DefaultAPIListenAddress    = "0.0.0.0"
	DefaultAPIPort             = 5199
	DefaultReadTimeout         = 5 * time.Second
	DefaultReadHeaderTimeout   = 5 * time.Second
	DefaultWriteTimeout        = 60 * time.Second
	DefaultShutdownTimeout     = 10 * time.Second
	DefaultRequestLogLevel     = "debug"
	DefaultMaxRequestBodyBytes = 32 * 1024 * 1024
)

func init() {
	SetDefault("api.listen_address", DefaultAPIListenAddress)
	SetDefault("api.port", DefaultAPIPort)
	SetDefault("api.read_timeout", DefaultReadTimeout)
	SetDefault("api.read_header_timeout", DefaultReadHeaderTimeout)
	SetDefault("api.write_timeout", DefaultWriteTimeout)
	SetDefault("api.shutdown_timeout", DefaultShutdownTimeout)
	SetDefault("api.request_log_level", DefaultRequestLogLevel)
	SetDefault("api.max_request_body_bytes", DefaultMaxRequestBodyBytes)
}
