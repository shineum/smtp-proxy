package queue

import (
	"math/rand/v2"
	"time"
)

// Default retry schedule durations.
var retrySchedule = []time.Duration{
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
}

// RetryStrategy implements exponential backoff with jitter for message retries.
type RetryStrategy struct {
	MaxRetries int
	Schedule   []time.Duration
}

// NewRetryStrategy creates a RetryStrategy with the default schedule and the
// given maximum retry count.
func NewRetryStrategy(maxRetries int) *RetryStrategy {
	return &RetryStrategy{
		MaxRetries: maxRetries,
		Schedule:   retrySchedule,
	}
}

// ShouldRetry returns true if the message has not exhausted its retry budget.
func (r *RetryStrategy) ShouldRetry(retryCount int) bool {
	return retryCount < r.MaxRetries
}

// NextBackoff returns the backoff duration for the given retry attempt with
// jitter applied. Jitter is calculated as: base * (0.5 + rand * 0.5).
func (r *RetryStrategy) NextBackoff(retryCount int) time.Duration {
	idx := retryCount
	if idx >= len(r.Schedule) {
		idx = len(r.Schedule) - 1
	}

	base := r.Schedule[idx]
	jitter := 0.5 + rand.Float64()*0.5
	return time.Duration(float64(base) * jitter)
}
