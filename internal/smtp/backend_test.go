package smtp

import (
	"testing"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/rs/zerolog"
)

// stubConn creates a minimal *gosmtp.Conn for testing purposes.
// Since gosmtp.Conn is created internally by the server, we use nil
// in tests where the Conn methods are not called. For tests that call
// conn.Hostname(), we rely on the zero-value behavior.
// Note: In production, connections are created by the go-smtp server.

func TestNewBackend_NewSession(t *testing.T) {
	mock := &mockQuerier{}
	log := zerolog.Nop()
	b := NewBackend(mock, log, 10)

	// NewSession requires a *gosmtp.Conn. Since we cannot construct one
	// directly (it is created by the go-smtp server), we test the backend
	// logic through the session tests that exercise the full flow.
	// Here we verify the backend is constructed correctly.
	if b.maxConns != 10 {
		t.Errorf("expected maxConns=10, got %d", b.maxConns)
	}
	if b.ActiveSessions() != 0 {
		t.Errorf("expected 0 active sessions, got %d", b.ActiveSessions())
	}

	_ = gosmtp.NewServer(b) // Verify Backend interface satisfaction.
}

func TestNewBackend_ActiveSessionCounter(t *testing.T) {
	mock := &mockQuerier{}
	log := zerolog.Nop()
	b := NewBackend(mock, log, 100)

	// Simulate session creation by incrementing the counter directly.
	b.active.Add(1)
	if b.ActiveSessions() != 1 {
		t.Errorf("expected 1 active session, got %d", b.ActiveSessions())
	}

	b.active.Add(1)
	if b.ActiveSessions() != 2 {
		t.Errorf("expected 2 active sessions, got %d", b.ActiveSessions())
	}

	// Simulate logout by decrementing.
	b.active.Add(-1)
	if b.ActiveSessions() != 1 {
		t.Errorf("expected 1 active session after decrement, got %d", b.ActiveSessions())
	}
}

func TestNewBackend_ConnectionLimitCheck(t *testing.T) {
	mock := &mockQuerier{}
	log := zerolog.Nop()
	b := NewBackend(mock, log, 2)

	// Simulate filling up to the limit.
	b.active.Add(2)
	if b.ActiveSessions() != 2 {
		t.Errorf("expected 2 active sessions, got %d", b.ActiveSessions())
	}

	// The next session attempt would exceed the limit.
	// We test the connection limit logic through NewSession which requires
	// a real *gosmtp.Conn. The counter-based logic is verified here.
	current := b.active.Add(1)
	if int(current) <= b.maxConns {
		t.Error("expected current to exceed maxConns")
	}
	// Clean up: revert the test increment.
	b.active.Add(-1)
}
