package queue

import (
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	tenantID := "tenant-abc"
	from := "sender@example.com"
	to := []string{"recipient1@example.com", "recipient2@example.com"}
	subject := "Test Subject"
	body := []byte("Hello, World!")

	before := time.Now()
	msg := NewMessage(tenantID, from, to, subject, body)
	after := time.Now()

	if msg == nil {
		t.Fatal("NewMessage() returned nil")
	}

	t.Run("ID is a valid UUID", func(t *testing.T) {
		if msg.ID == "" {
			t.Fatal("NewMessage() ID is empty")
		}
		// UUID v4 format: 8-4-4-4-12 hex characters
		if len(msg.ID) != 36 {
			t.Errorf("NewMessage() ID length = %d, want 36 (UUID format)", len(msg.ID))
		}
		// Verify dash positions
		if msg.ID[8] != '-' || msg.ID[13] != '-' || msg.ID[18] != '-' || msg.ID[23] != '-' {
			t.Errorf("NewMessage() ID = %q, does not match UUID dash pattern", msg.ID)
		}
	})

	t.Run("TenantID is set correctly", func(t *testing.T) {
		if msg.TenantID != tenantID {
			t.Errorf("NewMessage() TenantID = %q, want %q", msg.TenantID, tenantID)
		}
	})

	t.Run("From is set correctly", func(t *testing.T) {
		if msg.From != from {
			t.Errorf("NewMessage() From = %q, want %q", msg.From, from)
		}
	})

	t.Run("To is set correctly", func(t *testing.T) {
		if len(msg.To) != len(to) {
			t.Fatalf("NewMessage() To length = %d, want %d", len(msg.To), len(to))
		}
		for i, addr := range to {
			if msg.To[i] != addr {
				t.Errorf("NewMessage() To[%d] = %q, want %q", i, msg.To[i], addr)
			}
		}
	})

	t.Run("Subject is set correctly", func(t *testing.T) {
		if msg.Subject != subject {
			t.Errorf("NewMessage() Subject = %q, want %q", msg.Subject, subject)
		}
	})

	t.Run("Body is set correctly", func(t *testing.T) {
		if string(msg.Body) != string(body) {
			t.Errorf("NewMessage() Body = %q, want %q", msg.Body, body)
		}
	})

	t.Run("RetryCount is zero", func(t *testing.T) {
		if msg.RetryCount != 0 {
			t.Errorf("NewMessage() RetryCount = %d, want 0", msg.RetryCount)
		}
	})

	t.Run("CreatedAt is set to approximately now", func(t *testing.T) {
		if msg.CreatedAt.Before(before) {
			t.Errorf("NewMessage() CreatedAt %v is before test start %v", msg.CreatedAt, before)
		}
		if msg.CreatedAt.After(after) {
			t.Errorf("NewMessage() CreatedAt %v is after test end %v", msg.CreatedAt, after)
		}
	})

	t.Run("Headers is nil by default", func(t *testing.T) {
		if msg.Headers != nil {
			t.Errorf("NewMessage() Headers = %v, want nil", msg.Headers)
		}
	})
}

func TestNewMessage_UniqueIDs(t *testing.T) {
	msg1 := NewMessage("t1", "a@b.com", []string{"c@d.com"}, "s1", []byte("b1"))
	msg2 := NewMessage("t1", "a@b.com", []string{"c@d.com"}, "s1", []byte("b1"))

	if msg1.ID == msg2.ID {
		t.Errorf("NewMessage() generated duplicate IDs: %q", msg1.ID)
	}
}

func TestStreamKey(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		want     string
	}{
		{
			name:     "standard tenant ID",
			tenantID: "tenant-123",
			want:     "queue:tenant-123",
		},
		{
			name:     "empty tenant ID",
			tenantID: "",
			want:     "queue:",
		},
		{
			name:     "tenant ID with special characters",
			tenantID: "org:team-alpha",
			want:     "queue:org:team-alpha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamKey(tt.tenantID)
			if got != tt.want {
				t.Errorf("streamKey(%q) = %q, want %q", tt.tenantID, got, tt.want)
			}
		})
	}
}

func TestDlqStreamKey(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		want     string
	}{
		{
			name:     "standard tenant ID",
			tenantID: "tenant-123",
			want:     "dlq:tenant-123",
		},
		{
			name:     "empty tenant ID",
			tenantID: "",
			want:     "dlq:",
		},
		{
			name:     "tenant ID with special characters",
			tenantID: "org:team-alpha",
			want:     "dlq:org:team-alpha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dlqStreamKey(tt.tenantID)
			if got != tt.want {
				t.Errorf("dlqStreamKey(%q) = %q, want %q", tt.tenantID, got, tt.want)
			}
		})
	}
}
