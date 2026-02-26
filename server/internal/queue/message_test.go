package queue

import (
	"encoding/json"
	"strings"
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

// ---------------------------------------------------------------------------
// NewIDOnlyMessage
// ---------------------------------------------------------------------------

func TestNewIDOnlyMessage(t *testing.T) {
	id := "msg-uuid-123"
	accountID := "account-uuid-456"
	tenantID := "tenant-abc"

	before := time.Now()
	msg := NewIDOnlyMessage(id, accountID, tenantID)
	after := time.Now()

	if msg == nil {
		t.Fatal("NewIDOnlyMessage() returned nil")
	}

	t.Run("ID is set correctly", func(t *testing.T) {
		if msg.ID != id {
			t.Errorf("ID = %q, want %q", msg.ID, id)
		}
	})

	t.Run("AccountID is set correctly", func(t *testing.T) {
		if msg.AccountID != accountID {
			t.Errorf("AccountID = %q, want %q", msg.AccountID, accountID)
		}
	})

	t.Run("TenantID is set correctly", func(t *testing.T) {
		if msg.TenantID != tenantID {
			t.Errorf("TenantID = %q, want %q", msg.TenantID, tenantID)
		}
	})

	t.Run("CreatedAt is set to approximately now", func(t *testing.T) {
		if msg.CreatedAt.Before(before) || msg.CreatedAt.After(after) {
			t.Errorf("CreatedAt = %v, want between %v and %v", msg.CreatedAt, before, after)
		}
	})

	t.Run("payload fields are zero values", func(t *testing.T) {
		if msg.From != "" {
			t.Errorf("From = %q, want empty", msg.From)
		}
		if len(msg.To) != 0 {
			t.Errorf("To = %v, want empty", msg.To)
		}
		if msg.Subject != "" {
			t.Errorf("Subject = %q, want empty", msg.Subject)
		}
		if len(msg.Body) != 0 {
			t.Errorf("Body = %v, want empty", msg.Body)
		}
		if msg.RetryCount != 0 {
			t.Errorf("RetryCount = %d, want 0", msg.RetryCount)
		}
		if msg.Headers != nil {
			t.Errorf("Headers = %v, want nil", msg.Headers)
		}
	})
}

// ---------------------------------------------------------------------------
// HasInlineBody
// ---------------------------------------------------------------------------

func TestHasInlineBody(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want bool
	}{
		{
			name: "full payload message with body",
			msg:  Message{Body: []byte("Hello, World!")},
			want: true,
		},
		{
			name: "ID-only message with nil body",
			msg:  Message{ID: "id-1", AccountID: "acct-1", TenantID: "t1"},
			want: false,
		},
		{
			name: "message with empty body slice",
			msg:  Message{Body: []byte{}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.HasInlineBody()
			if got != tt.want {
				t.Errorf("HasInlineBody() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JSON serialization (new ID-only format)
// ---------------------------------------------------------------------------

func TestMessage_JSON_IDOnly_Serialization(t *testing.T) {
	msg := NewIDOnlyMessage("msg-1", "acct-1", "tenant-1")

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	raw := string(data)

	// Required fields must be present.
	for _, key := range []string{`"id"`, `"account_id"`, `"tenant_id"`, `"retry_count"`, `"created_at"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON output missing required key %s: %s", key, raw)
		}
	}

	// Payload fields must be omitted.
	for _, key := range []string{`"from"`, `"to"`, `"subject"`, `"body"`, `"headers"`} {
		if strings.Contains(raw, key) {
			t.Errorf("JSON output should omit %s for ID-only message: %s", key, raw)
		}
	}
}

func TestMessage_JSON_FullPayload_Serialization(t *testing.T) {
	msg := NewMessage("tenant-1", "a@b.com", []string{"c@d.com"}, "Hi", []byte("body"))

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	raw := string(data)

	// All payload fields must be present.
	for _, key := range []string{`"id"`, `"tenant_id"`, `"from"`, `"to"`, `"subject"`, `"body"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON output missing key %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON deserialization (both formats)
// ---------------------------------------------------------------------------

func TestMessage_JSON_Deserialization_IDOnly(t *testing.T) {
	input := `{"id":"msg-1","account_id":"acct-1","tenant_id":"t1","retry_count":0,"created_at":"2025-01-01T00:00:00Z"}`

	var msg Message
	if err := json.Unmarshal([]byte(input), &msg); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if msg.ID != "msg-1" {
		t.Errorf("ID = %q, want %q", msg.ID, "msg-1")
	}
	if msg.AccountID != "acct-1" {
		t.Errorf("AccountID = %q, want %q", msg.AccountID, "acct-1")
	}
	if msg.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", msg.TenantID, "t1")
	}
	if msg.From != "" {
		t.Errorf("From = %q, want empty", msg.From)
	}
	if len(msg.To) != 0 {
		t.Errorf("To = %v, want empty", msg.To)
	}
	if len(msg.Body) != 0 {
		t.Errorf("Body = %v, want empty", msg.Body)
	}
	if msg.HasInlineBody() {
		t.Error("HasInlineBody() = true, want false for ID-only message")
	}
}

func TestMessage_JSON_Deserialization_FullPayload(t *testing.T) {
	input := `{"id":"msg-2","tenant_id":"t1","from":"a@b.com","to":["c@d.com"],"subject":"Hi","body":"Ym9keQ==","retry_count":2,"created_at":"2025-01-01T00:00:00Z"}`

	var msg Message
	if err := json.Unmarshal([]byte(input), &msg); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if msg.ID != "msg-2" {
		t.Errorf("ID = %q, want %q", msg.ID, "msg-2")
	}
	if msg.AccountID != "" {
		t.Errorf("AccountID = %q, want empty (legacy format)", msg.AccountID)
	}
	if msg.From != "a@b.com" {
		t.Errorf("From = %q, want %q", msg.From, "a@b.com")
	}
	if len(msg.To) != 1 || msg.To[0] != "c@d.com" {
		t.Errorf("To = %v, want [c@d.com]", msg.To)
	}
	if msg.Subject != "Hi" {
		t.Errorf("Subject = %q, want %q", msg.Subject, "Hi")
	}
	if string(msg.Body) != "body" {
		t.Errorf("Body = %q, want %q", msg.Body, "body")
	}
	if msg.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", msg.RetryCount)
	}
	if !msg.HasInlineBody() {
		t.Error("HasInlineBody() = false, want true for full payload message")
	}
}

// ---------------------------------------------------------------------------
// Round-trip: marshal then unmarshal preserves data
// ---------------------------------------------------------------------------

func TestMessage_JSON_RoundTrip_FullPayload(t *testing.T) {
	original := NewMessage("t1", "sender@x.com", []string{"r1@x.com", "r2@x.com"}, "Subject", []byte("email body"))
	original.Headers = map[string]string{"X-Custom": "value"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.From != original.From {
		t.Errorf("From = %q, want %q", decoded.From, original.From)
	}
	if string(decoded.Body) != string(original.Body) {
		t.Errorf("Body = %q, want %q", decoded.Body, original.Body)
	}
	if decoded.Headers["X-Custom"] != "value" {
		t.Errorf("Headers[X-Custom] = %q, want %q", decoded.Headers["X-Custom"], "value")
	}
}

func TestMessage_JSON_RoundTrip_IDOnly(t *testing.T) {
	original := NewIDOnlyMessage("msg-id", "acct-id", "t1")

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.AccountID != original.AccountID {
		t.Errorf("AccountID = %q, want %q", decoded.AccountID, original.AccountID)
	}
	if decoded.TenantID != original.TenantID {
		t.Errorf("TenantID = %q, want %q", decoded.TenantID, original.TenantID)
	}
	if decoded.HasInlineBody() {
		t.Error("HasInlineBody() = true after round-trip, want false")
	}
}
