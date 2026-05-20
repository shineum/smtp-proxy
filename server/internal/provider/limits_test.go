package provider

import "testing"

func TestMaxMessageBytes_KnownProviders(t *testing.T) {
	tests := []struct {
		name string
		want int64
	}{
		{"sendgrid", 30 * 1024 * 1024},
		{"ses", 40 * 1024 * 1024},
		{"mailgun", 25 * 1024 * 1024},
		{"msgraph", 4 * 1024 * 1024},
		{"smtp", 25 * 1024 * 1024},
	}
	for _, tt := range tests {
		if got := MaxMessageBytes(tt.name); got != tt.want {
			t.Errorf("MaxMessageBytes(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestMaxMessageBytes_UnknownReturnsZero(t *testing.T) {
	if got := MaxMessageBytes("postmark"); got != 0 {
		t.Errorf("MaxMessageBytes(unknown) = %d, want 0 (no limit)", got)
	}
	if got := MaxMessageBytes(""); got != 0 {
		t.Errorf("MaxMessageBytes(empty) = %d, want 0", got)
	}
}

func TestEstimateMessageBytes_NilMessage(t *testing.T) {
	if got := EstimateMessageBytes(nil); got != 0 {
		t.Errorf("EstimateMessageBytes(nil) = %d, want 0", got)
	}
}

func TestEstimateMessageBytes_TextOnly(t *testing.T) {
	msg := &Message{
		Subject:  "hello",
		TextBody: "world",
	}
	got := EstimateMessageBytes(msg)
	// Body+text+subject = 10; overhead ~512; should be >= 512 and < 1024.
	if got < 512 || got > 1024 {
		t.Errorf("EstimateMessageBytes(text only) = %d, want roughly 522", got)
	}
}

func TestEstimateMessageBytes_AttachmentBase64Inflation(t *testing.T) {
	// 3 MB raw attachment → ~4 MB base64-encoded.
	raw := make([]byte, 3*1024*1024)
	msg := &Message{
		Attachments: []Attachment{
			{Filename: "big.pdf", ContentType: "application/pdf", Content: raw},
		},
	}
	got := EstimateMessageBytes(msg)
	// Expect at least 4 MB (base64 inflation) plus some headers.
	if got < 4*1024*1024 {
		t.Errorf("EstimateMessageBytes(3MB raw) = %d, want >= 4MB after base64", got)
	}
	// And the inflation should be close to 4MB, not 12MB or anything wild.
	if got > 5*1024*1024 {
		t.Errorf("EstimateMessageBytes(3MB raw) = %d, want roughly 4MB", got)
	}
}

func TestEstimateMessageBytes_ExceedsMSGraphLimit(t *testing.T) {
	// 5 MB raw attachment will exceed MS Graph's 4 MB sendMail cap even before
	// base64; this is the realistic failure case REQ-MIME-006 must catch.
	raw := make([]byte, 5*1024*1024)
	msg := &Message{
		Attachments: []Attachment{{Filename: "big.pdf", ContentType: "application/pdf", Content: raw}},
	}
	got := EstimateMessageBytes(msg)
	if got <= MaxMessageBytes("msgraph") {
		t.Errorf("5MB attachment estimate = %d should exceed msgraph limit %d", got, MaxMessageBytes("msgraph"))
	}
}
