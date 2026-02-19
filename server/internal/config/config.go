package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	SMTP      SMTPConfig      `mapstructure:"smtp"`
	API       APIConfig       `mapstructure:"api"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	TLS       TLSConfig       `mapstructure:"tls"`
	Delivery  DeliveryConfig  `mapstructure:"delivery"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Auth      AuthConfig      `mapstructure:"auth"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

// AuthConfig holds JWT authentication configuration.
type AuthConfig struct {
	// SigningKey is the HMAC secret key for signing JWT tokens.
	// MUST be set via environment variable SMTP_PROXY_AUTH_SIGNING_KEY in production.
	SigningKey string `mapstructure:"signing_key"`
	// AccessTokenExpiry is the duration an access token remains valid.
	AccessTokenExpiry time.Duration `mapstructure:"access_token_expiry"`
	// RefreshTokenExpiry is the duration a refresh token remains valid.
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	// Issuer is the JWT issuer claim.
	Issuer string `mapstructure:"issuer"`
	// Audience is the JWT audience claim.
	Audience string `mapstructure:"audience"`
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// DefaultMonthlyLimit is the default monthly email limit per tenant.
	DefaultMonthlyLimit int `mapstructure:"default_monthly_limit"`
	// LoginAttemptsLimit is the max failed login attempts before lockout.
	LoginAttemptsLimit int `mapstructure:"login_attempts_limit"`
	// LoginLockoutDuration is how long a user is locked out after exceeding attempts.
	LoginLockoutDuration time.Duration `mapstructure:"login_lockout_duration"`
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

// DeliveryConfig holds message delivery configuration.
type DeliveryConfig struct {
	// Mode selects the delivery strategy: "sync" or "async".
	// Sync delivers via ESP provider inline after DB insert (no Redis needed).
	// Async enqueues to Redis Streams for background worker delivery.
	Mode string `mapstructure:"mode"`
}

// QueueConfig holds Redis-based queue configuration for async delivery mode.
type QueueConfig struct {
	RedisAddr     string        `mapstructure:"redis_addr"`
	RedisPassword string        `mapstructure:"redis_password"`
	RedisDB       int           `mapstructure:"redis_db"`
	StreamName    string        `mapstructure:"stream_name"`
	GroupName     string        `mapstructure:"group_name"`
	ConsumerID    string        `mapstructure:"consumer_id"`
	Workers       int           `mapstructure:"workers"`
	BlockTimeout  time.Duration `mapstructure:"block_timeout"`
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

	// Set defaults for delivery and queue configuration.
	v.SetDefault("delivery.mode", "sync")
	v.SetDefault("queue.redis_addr", "localhost:6379")
	v.SetDefault("queue.redis_db", 0)
	v.SetDefault("queue.stream_name", "smtp-proxy")
	v.SetDefault("queue.group_name", "workers")
	v.SetDefault("queue.consumer_id", "worker-1")
	v.SetDefault("queue.workers", 10)
	v.SetDefault("queue.block_timeout", "5s")

	// Set defaults for auth configuration.
	v.SetDefault("auth.signing_key", "")
	v.SetDefault("auth.access_token_expiry", "15m")
	v.SetDefault("auth.refresh_token_expiry", "168h") // 7 days
	v.SetDefault("auth.issuer", "smtp-proxy")
	v.SetDefault("auth.audience", "smtp-proxy-api")

	// Set defaults for rate limiting configuration.
	v.SetDefault("rate_limit.default_monthly_limit", 10000)
	v.SetDefault("rate_limit.login_attempts_limit", 5)
	v.SetDefault("rate_limit.login_lockout_duration", "15m")

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
