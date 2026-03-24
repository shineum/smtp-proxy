package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DomainRateLimitResult holds the outcome of a domain rate limit check.
type DomainRateLimitResult struct {
	Allowed bool
	// RetryAfter is the suggested delay before retrying when rate-limited.
	RetryAfter time.Duration
	// Reason describes why the request was rate-limited.
	Reason string
}

// DomainRateLimiter enforces per-destination-domain sending limits using Redis
// sliding windows. This prevents receiving mail servers from classifying
// outbound emails as spam due to burst sending patterns.
type DomainRateLimiter struct {
	client *redis.Client
}

// NewDomainRateLimiter creates a DomainRateLimiter backed by the given Redis client.
func NewDomainRateLimiter(client *redis.Client) *DomainRateLimiter {
	return &DomainRateLimiter{client: client}
}

// CheckDomainRateLimit checks whether sending to the given domain is allowed
// under the configured per-minute and per-hour limits for the group.
// A limit value of 0 means unlimited.
func (d *DomainRateLimiter) CheckDomainRateLimit(ctx context.Context, groupID uuid.UUID, domain string, maxPerMinute, maxPerHour int32) DomainRateLimitResult {
	if d.client == nil {
		return DomainRateLimitResult{Allowed: true}
	}

	// Check per-minute limit.
	if maxPerMinute > 0 {
		key := fmt.Sprintf("ratelimit:domain:%s:%s:min", groupID.String(), domain)
		count, err := d.client.Get(ctx, key).Int64()
		if err != nil && err != redis.Nil {
			// On Redis error, allow the send (fail open).
			return DomainRateLimitResult{Allowed: true}
		}
		if count >= int64(maxPerMinute) {
			return DomainRateLimitResult{
				Allowed:    false,
				RetryAfter: 30 * time.Second,
				Reason:     fmt.Sprintf("per-minute limit reached for %s (%d/%d)", domain, count, maxPerMinute),
			}
		}
	}

	// Check per-hour limit.
	if maxPerHour > 0 {
		key := fmt.Sprintf("ratelimit:domain:%s:%s:hour", groupID.String(), domain)
		count, err := d.client.Get(ctx, key).Int64()
		if err != nil && err != redis.Nil {
			return DomainRateLimitResult{Allowed: true}
		}
		if count >= int64(maxPerHour) {
			return DomainRateLimitResult{
				Allowed:    false,
				RetryAfter: 5 * time.Minute,
				Reason:     fmt.Sprintf("per-hour limit reached for %s (%d/%d)", domain, count, maxPerHour),
			}
		}
	}

	return DomainRateLimitResult{Allowed: true}
}

// IncrementDomainCount increments the send counters for the given domain.
func (d *DomainRateLimiter) IncrementDomainCount(ctx context.Context, groupID uuid.UUID, domain string) error {
	if d.client == nil {
		return nil
	}

	pipe := d.client.Pipeline()

	minKey := fmt.Sprintf("ratelimit:domain:%s:%s:min", groupID.String(), domain)
	pipe.Incr(ctx, minKey)
	pipe.Expire(ctx, minKey, 1*time.Minute)

	hourKey := fmt.Sprintf("ratelimit:domain:%s:%s:hour", groupID.String(), domain)
	pipe.Incr(ctx, hourKey)
	pipe.Expire(ctx, hourKey, 1*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("increment domain count for %s: %w", domain, err)
	}

	return nil
}
