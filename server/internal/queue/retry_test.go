package queue

import (
	"testing"
	"time"
)

func TestNewRetryStrategy(t *testing.T) {
	rs := NewRetryStrategy(5)

	t.Run("MaxRetries is set correctly", func(t *testing.T) {
		if rs.MaxRetries != 5 {
			t.Errorf("NewRetryStrategy(5) MaxRetries = %d, want 5", rs.MaxRetries)
		}
	})

	t.Run("Schedule uses default retrySchedule", func(t *testing.T) {
		expectedSchedule := []time.Duration{
			30 * time.Second,
			1 * time.Minute,
			2 * time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		}
		if len(rs.Schedule) != len(expectedSchedule) {
			t.Fatalf("NewRetryStrategy() Schedule length = %d, want %d", len(rs.Schedule), len(expectedSchedule))
		}
		for i, want := range expectedSchedule {
			if rs.Schedule[i] != want {
				t.Errorf("NewRetryStrategy() Schedule[%d] = %v, want %v", i, rs.Schedule[i], want)
			}
		}
	})
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		retryCount int
		want       bool
	}{
		{
			name:       "first retry attempt",
			maxRetries: 5,
			retryCount: 0,
			want:       true,
		},
		{
			name:       "mid-range retry attempt",
			maxRetries: 5,
			retryCount: 3,
			want:       true,
		},
		{
			name:       "last allowed retry",
			maxRetries: 5,
			retryCount: 4,
			want:       true,
		},
		{
			name:       "retry count equals max retries",
			maxRetries: 5,
			retryCount: 5,
			want:       false,
		},
		{
			name:       "retry count exceeds max retries",
			maxRetries: 5,
			retryCount: 10,
			want:       false,
		},
		{
			name:       "zero max retries with zero count",
			maxRetries: 0,
			retryCount: 0,
			want:       false,
		},
		{
			name:       "single retry allowed - not yet used",
			maxRetries: 1,
			retryCount: 0,
			want:       true,
		},
		{
			name:       "single retry allowed - already used",
			maxRetries: 1,
			retryCount: 1,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := NewRetryStrategy(tt.maxRetries)
			got := rs.ShouldRetry(tt.retryCount)
			if got != tt.want {
				t.Errorf("ShouldRetry(%d) with maxRetries=%d: got %v, want %v",
					tt.retryCount, tt.maxRetries, got, tt.want)
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	// The schedule is: [30s, 1m, 2m, 5m, 15m]
	// Jitter formula: base * (0.5 + rand * 0.5)
	// This means the result is in the range [base*0.5, base*1.0]
	tests := []struct {
		name       string
		retryCount int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "first retry (30s base)",
			retryCount: 0,
			wantMin:    15 * time.Second,
			wantMax:    30 * time.Second,
		},
		{
			name:       "second retry (1m base)",
			retryCount: 1,
			wantMin:    30 * time.Second,
			wantMax:    1 * time.Minute,
		},
		{
			name:       "third retry (2m base)",
			retryCount: 2,
			wantMin:    1 * time.Minute,
			wantMax:    2 * time.Minute,
		},
		{
			name:       "fourth retry (5m base)",
			retryCount: 3,
			wantMin:    150 * time.Second, // 2.5 minutes
			wantMax:    5 * time.Minute,
		},
		{
			name:       "fifth retry (15m base)",
			retryCount: 4,
			wantMin:    450 * time.Second, // 7.5 minutes
			wantMax:    15 * time.Minute,
		},
		{
			name:       "beyond schedule length uses last entry (15m base)",
			retryCount: 5,
			wantMin:    450 * time.Second, // 7.5 minutes
			wantMax:    15 * time.Minute,
		},
		{
			name:       "far beyond schedule length uses last entry (15m base)",
			retryCount: 100,
			wantMin:    450 * time.Second, // 7.5 minutes
			wantMax:    15 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := NewRetryStrategy(10)
			// Run multiple times to verify jitter stays within range
			for i := 0; i < 100; i++ {
				got := rs.NextBackoff(tt.retryCount)
				if got < tt.wantMin {
					t.Errorf("NextBackoff(%d) iteration %d: got %v, below minimum %v",
						tt.retryCount, i, got, tt.wantMin)
				}
				if got > tt.wantMax {
					t.Errorf("NextBackoff(%d) iteration %d: got %v, above maximum %v",
						tt.retryCount, i, got, tt.wantMax)
				}
			}
		})
	}
}

func TestNextBackoff_ProducesVariation(t *testing.T) {
	rs := NewRetryStrategy(5)
	seen := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		d := rs.NextBackoff(0)
		seen[d] = true
	}
	// With jitter, we expect more than one unique duration across 100 calls.
	// It is statistically near impossible to get the same value 100 times.
	if len(seen) < 2 {
		t.Errorf("NextBackoff() produced %d unique values over 100 calls, expected variation from jitter", len(seen))
	}
}
