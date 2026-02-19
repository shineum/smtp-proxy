package auth

import (
	"testing"
	"time"
)

func TestCurrentMonth(t *testing.T) {
	month := currentMonth()
	if len(month) != 7 {
		t.Errorf("currentMonth() = %q, expected format YYYY-MM (length 7)", month)
	}
}

func TestDaysUntilEndOfMonth(t *testing.T) {
	d := daysUntilEndOfMonth()
	if d <= 0 {
		t.Errorf("daysUntilEndOfMonth() = %v, expected positive duration", d)
	}
	if d > 31*24*time.Hour {
		t.Errorf("daysUntilEndOfMonth() = %v, expected less than 31 days", d)
	}
}

func TestNewRateLimiter_NilClient(t *testing.T) {
	config := RateLimitConfig{
		DefaultMonthlyLimit:  10000,
		LoginAttemptsLimit:   5,
		LoginLockoutDuration: 15 * time.Minute,
	}

	rl := NewRateLimiter(nil, config)
	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}

	// All methods should gracefully handle nil client
	ctx := t.Context()
	if err := rl.CheckSMTPRateLimit(ctx, [16]byte{}, 10000); err != nil {
		t.Errorf("CheckSMTPRateLimit() with nil client error = %v", err)
	}
	if err := rl.IncrementSMTPCount(ctx, [16]byte{}); err != nil {
		t.Errorf("IncrementSMTPCount() with nil client error = %v", err)
	}
	if err := rl.CheckLoginRateLimit(ctx, "test@example.com"); err != nil {
		t.Errorf("CheckLoginRateLimit() with nil client error = %v", err)
	}
	if err := rl.RecordFailedLogin(ctx, "test@example.com"); err != nil {
		t.Errorf("RecordFailedLogin() with nil client error = %v", err)
	}
	if err := rl.ClearFailedLogins(ctx, "test@example.com"); err != nil {
		t.Errorf("ClearFailedLogins() with nil client error = %v", err)
	}
}
