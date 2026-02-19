package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// DefaultMonthlyLimit is the default monthly email limit per tenant.
	DefaultMonthlyLimit int `mapstructure:"default_monthly_limit"`
	// LoginAttemptsLimit is the max failed login attempts before lockout.
	LoginAttemptsLimit int `mapstructure:"login_attempts_limit"`
	// LoginLockoutDuration is how long a user is locked out after exceeding attempts.
	LoginLockoutDuration time.Duration `mapstructure:"login_lockout_duration"`
}

// RateLimiter provides per-tenant rate limiting using Redis sliding window.
type RateLimiter struct {
	client *redis.Client
	config RateLimitConfig
}

// NewRateLimiter creates a new RateLimiter with the given Redis client and configuration.
func NewRateLimiter(client *redis.Client, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		client: client,
		config: config,
	}
}

// CheckSMTPRateLimit checks whether the given tenant has exceeded their monthly send limit.
// Returns nil if allowed, or an error if the rate limit is exceeded.
func (rl *RateLimiter) CheckSMTPRateLimit(ctx context.Context, tenantID uuid.UUID, monthlyLimit int) error {
	if rl.client == nil {
		// No Redis client configured; skip rate limiting.
		return nil
	}

	key := fmt.Sprintf("ratelimit:smtp:%s:%s", tenantID.String(), currentMonth())
	count, err := rl.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("check rate limit: %w", err)
	}

	if int(count) >= monthlyLimit {
		return fmt.Errorf("monthly send limit exceeded (%d/%d)", count, monthlyLimit)
	}

	return nil
}

// IncrementSMTPCount increments the monthly send counter for the given tenant.
func (rl *RateLimiter) IncrementSMTPCount(ctx context.Context, tenantID uuid.UUID) error {
	if rl.client == nil {
		return nil
	}

	key := fmt.Sprintf("ratelimit:smtp:%s:%s", tenantID.String(), currentMonth())

	pipe := rl.client.Pipeline()
	pipe.Incr(ctx, key)
	// Set expiry to end of current month + 1 day buffer
	pipe.Expire(ctx, key, daysUntilEndOfMonth()+24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("increment smtp count: %w", err)
	}

	return nil
}

// CheckLoginRateLimit checks whether a user has exceeded the login attempt limit.
// Returns nil if allowed, or an error if locked out.
func (rl *RateLimiter) CheckLoginRateLimit(ctx context.Context, email string) error {
	if rl.client == nil {
		return nil
	}

	key := fmt.Sprintf("ratelimit:login:%s", email)
	count, err := rl.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("check login rate limit: %w", err)
	}

	if int(count) >= rl.config.LoginAttemptsLimit {
		return fmt.Errorf("account temporarily locked due to too many failed login attempts")
	}

	return nil
}

// RecordFailedLogin increments the failed login counter for the given email.
func (rl *RateLimiter) RecordFailedLogin(ctx context.Context, email string) error {
	if rl.client == nil {
		return nil
	}

	key := fmt.Sprintf("ratelimit:login:%s", email)

	pipe := rl.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rl.config.LoginLockoutDuration)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("record failed login: %w", err)
	}

	return nil
}

// ClearFailedLogins resets the failed login counter for the given email.
func (rl *RateLimiter) ClearFailedLogins(ctx context.Context, email string) error {
	if rl.client == nil {
		return nil
	}

	key := fmt.Sprintf("ratelimit:login:%s", email)
	return rl.client.Del(ctx, key).Err()
}

// currentMonth returns the current year-month string (e.g., "2026-02").
func currentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

// daysUntilEndOfMonth returns the duration from now until the end of the current month.
func daysUntilEndOfMonth() time.Duration {
	now := time.Now().UTC()
	year, month, _ := now.Date()
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	return firstOfNext.Sub(now)
}
