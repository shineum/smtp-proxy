package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	SMTP     SMTPConfig     `mapstructure:"smtp"`
	API      APIConfig      `mapstructure:"api"`
	Database DatabaseConfig `mapstructure:"database"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	TLS      TLSConfig      `mapstructure:"tls"`
}

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	MaxConnections int           `mapstructure:"max_connections"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxMessageSize int64         `mapstructure:"max_message_size"`
}

// APIConfig holds REST API server configuration.
type APIConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig holds PostgreSQL connection configuration.
type DatabaseConfig struct {
	URL            string        `mapstructure:"url"`
	PoolMin        int32         `mapstructure:"pool_min"`
	PoolMax        int32         `mapstructure:"pool_max"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// TLSConfig holds TLS certificate configuration.
type TLSConfig struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

// Load reads configuration from the given config directory path.
// It looks for a file named "config.yaml" in that directory.
// Environment variables with prefix SMTP_PROXY_ override file values.
// For example, SMTP_PROXY_DATABASE_URL overrides database.url.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)

	v.SetEnvPrefix("SMTP_PROXY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
