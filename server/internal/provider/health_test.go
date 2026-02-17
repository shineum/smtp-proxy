package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockHealthProvider implements Provider with configurable HealthCheck behavior.
type mockHealthProvider struct {
	name string
	err  error
}

func (m *mockHealthProvider) Send(_ context.Context, _ *Message) (*DeliveryResult, error) {
	return nil, nil
}

func (m *mockHealthProvider) GetName() string {
	return m.name
}

func (m *mockHealthProvider) HealthCheck(_ context.Context) error {
	return m.err
}

func TestNewHealthChecker(t *testing.T) {
	r := NewRegistry()
	hc := NewHealthChecker(r)
	if hc == nil {
		t.Fatal("NewHealthChecker() returned nil")
	}

	statuses := hc.GetAllStatuses()
	if len(statuses) != 0 {
		t.Errorf("expected empty statuses, got %d", len(statuses))
	}
}

func TestHealthChecker_IsHealthy_UnknownProvider(t *testing.T) {
	r := NewRegistry()
	hc := NewHealthChecker(r)

	if hc.IsHealthy("nonexistent") {
		t.Error("expected IsHealthy to return false for unknown provider")
	}
}

func TestHealthChecker_HealthyAfterSuccessfulCheck(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "test-provider", err: nil}
	r.Register(mp)

	hc := NewHealthChecker(r)
	// Manually trigger a check cycle instead of using Start/Stop with timers.
	hc.checkAll()

	if !hc.IsHealthy("test-provider") {
		t.Error("expected IsHealthy to return true after successful check")
	}
}

func TestHealthChecker_UnhealthyAfterThreeFailures(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "failing", err: errors.New("connection refused")}
	r.Register(mp)

	hc := NewHealthChecker(r)

	// Three consecutive failures should mark provider unhealthy.
	hc.checkAll() // failure 1
	hc.checkAll() // failure 2
	hc.checkAll() // failure 3

	if hc.IsHealthy("failing") {
		t.Error("expected IsHealthy to return false after 3 consecutive failures")
	}
}

func TestHealthChecker_StillHealthyAfterTwoFailures(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "flaky", err: errors.New("timeout")}
	r.Register(mp)

	hc := NewHealthChecker(r)

	// Two failures should not mark unhealthy (threshold is 3).
	hc.checkAll() // failure 1
	hc.checkAll() // failure 2

	if !hc.IsHealthy("flaky") {
		t.Error("expected IsHealthy to return true after only 2 failures (threshold is 3)")
	}
}

func TestHealthChecker_RecoveryAfterFailures(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "recovering", err: errors.New("down")}
	r.Register(mp)

	hc := NewHealthChecker(r)

	// Mark as unhealthy.
	hc.checkAll() // failure 1
	hc.checkAll() // failure 2
	hc.checkAll() // failure 3

	if hc.IsHealthy("recovering") {
		t.Fatal("precondition: expected unhealthy after 3 failures")
	}

	// One success should reset to healthy.
	mp.err = nil
	hc.checkAll()

	if !hc.IsHealthy("recovering") {
		t.Error("expected IsHealthy to return true after 1 success following failures")
	}
}

func TestHealthChecker_GetStatus(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "status-test", err: nil}
	r.Register(mp)

	hc := NewHealthChecker(r)
	hc.checkAll()

	status, ok := hc.GetStatus("status-test")
	if !ok {
		t.Fatal("expected GetStatus to return true for registered provider")
	}
	if !status.Healthy {
		t.Error("expected status.Healthy to be true")
	}
	if status.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", status.ConsecutiveFailures)
	}
	if status.LastCheck.IsZero() {
		t.Error("expected non-zero LastCheck timestamp")
	}
	if status.LastError != "" {
		t.Errorf("expected empty LastError, got %q", status.LastError)
	}
}

func TestHealthChecker_GetStatus_Unknown(t *testing.T) {
	r := NewRegistry()
	hc := NewHealthChecker(r)

	_, ok := hc.GetStatus("nonexistent")
	if ok {
		t.Error("expected GetStatus to return false for unknown provider")
	}
}

func TestHealthChecker_GetStatus_WithFailures(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "error-status", err: errors.New("connection refused")}
	r.Register(mp)

	hc := NewHealthChecker(r)
	hc.checkAll()
	hc.checkAll()

	status, ok := hc.GetStatus("error-status")
	if !ok {
		t.Fatal("expected GetStatus to return true for registered provider")
	}
	if status.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", status.ConsecutiveFailures)
	}
	if status.LastError != "connection refused" {
		t.Errorf("expected LastError %q, got %q", "connection refused", status.LastError)
	}
	// Still healthy since threshold (3) not reached.
	if !status.Healthy {
		t.Error("expected status.Healthy to be true (only 2 failures, threshold is 3)")
	}
}

func TestHealthChecker_GetAllStatuses(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockHealthProvider{name: "provider-a", err: nil})
	r.Register(&mockHealthProvider{name: "provider-b", err: nil})

	hc := NewHealthChecker(r)
	hc.checkAll()

	statuses := hc.GetAllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	if _, ok := statuses["provider-a"]; !ok {
		t.Error("missing status for provider-a")
	}
	if _, ok := statuses["provider-b"]; !ok {
		t.Error("missing status for provider-b")
	}
}

func TestHealthChecker_StartAndStop(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockHealthProvider{name: "timed", err: nil})

	hc := NewHealthChecker(r)
	// Use a very short interval to ensure the check runs.
	hc.checkInterval = 10 * time.Millisecond

	hc.Start()

	// Wait for at least one check cycle.
	time.Sleep(50 * time.Millisecond)

	hc.Stop()

	if !hc.IsHealthy("timed") {
		t.Error("expected provider to be healthy after start/stop cycle")
	}
}

func TestHealthChecker_GetAllStatuses_ReturnsSnapshot(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "snapshot-test", err: nil}
	r.Register(mp)

	hc := NewHealthChecker(r)
	hc.checkAll()

	snapshot := hc.GetAllStatuses()
	// Mutating the snapshot should not affect the internal state.
	snapshot["snapshot-test"] = HealthStatus{Healthy: false}

	if !hc.IsHealthy("snapshot-test") {
		t.Error("mutating snapshot should not affect internal health state")
	}
}

func TestHealthChecker_ConsecutiveFailuresResetOnSuccess(t *testing.T) {
	r := NewRegistry()
	mp := &mockHealthProvider{name: "reset-test", err: errors.New("error")}
	r.Register(mp)

	hc := NewHealthChecker(r)
	hc.checkAll() // failure 1
	hc.checkAll() // failure 2

	// Recover.
	mp.err = nil
	hc.checkAll() // success

	status, _ := hc.GetStatus("reset-test")
	if status.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive failures reset to 0, got %d", status.ConsecutiveFailures)
	}
	if status.LastError != "" {
		t.Errorf("expected empty LastError after recovery, got %q", status.LastError)
	}
}
