package mimeparse

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestParse_PlainTextOnly(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Hello\r\n" +
		"\r\n" +
		"This is a plain text message.\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Subject != "Hello" {
		t.Errorf("subject = %q, want %q", msg.Subject, "Hello")
	}
	if msg.TextBody != "This is a plain text message.\r\n" {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "This is a plain text message.\r\n")
	}
	if msg.HTMLBody != "" {
		t.Errorf("HTMLBody should be empty, got %q", msg.HTMLBody)
	}
	if len(msg.Attachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(msg.Attachments))
	}
}

func TestParse_HTMLOnly(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: HTML Email\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body><h1>Hello</h1></body></html>\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Subject != "HTML Email" {
		t.Errorf("subject = %q, want %q", msg.Subject, "HTML Email")
	}
	if msg.TextBody != "" {
		t.Errorf("TextBody should be empty, got %q", msg.TextBody)
	}
	if msg.HTMLBody != "<html><body><h1>Hello</h1></body></html>\r\n" {
		t.Errorf("HTMLBody = %q, want %q", msg.HTMLBody, "<html><body><h1>Hello</h1></body></html>\r\n")
	}
}

func TestParse_MultipartAlternative(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Multipart Alternative\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"boundary-alt-001\"\r\n" +
		"\r\n" +
		"--boundary-alt-001\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain text version.\r\n" +
		"--boundary-alt-001\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body><p>HTML version.</p></body></html>\r\n" +
		"--boundary-alt-001--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// mime/multipart.Reader strips the trailing \r\n before the boundary delimiter.
	if msg.TextBody != "Plain text version." {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "Plain text version.")
	}
	if msg.HTMLBody != "<html><body><p>HTML version.</p></body></html>" {
		t.Errorf("HTMLBody = %q, want %q", msg.HTMLBody, "<html><body><p>HTML version.</p></body></html>")
	}
	if len(msg.Attachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(msg.Attachments))
	}
}

func TestParse_MultipartMixedWithTextAttachment(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: With Attachment\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary-mix-001\"\r\n" +
		"\r\n" +
		"--boundary-mix-001\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Message body here.\r\n" +
		"--boundary-mix-001\r\n" +
		"Content-Type: text/plain; charset=utf-8; name=\"notes.txt\"\r\n" +
		"Content-Disposition: attachment; filename=\"notes.txt\"\r\n" +
		"\r\n" +
		"These are the notes.\r\n" +
		"--boundary-mix-001--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != "Message body here." {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "Message body here.")
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}

	att := msg.Attachments[0]
	if att.Filename != "notes.txt" {
		t.Errorf("attachment filename = %q, want %q", att.Filename, "notes.txt")
	}
	if string(att.Content) != "These are the notes." {
		t.Errorf("attachment content = %q, want %q", string(att.Content), "These are the notes.")
	}
	if att.IsInline {
		t.Error("attachment should not be inline")
	}
}

func TestParse_MultipartMixedWithBinaryAttachment(t *testing.T) {
	// Simulate a small PNG-like binary payload.
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02, 0x03}
	encoded := base64.StdEncoding.EncodeToString(binaryData)

	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Binary Attachment\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary-bin-001\"\r\n" +
		"\r\n" +
		"--boundary-bin-001\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"See attached image.\r\n" +
		"--boundary-bin-001\r\n" +
		"Content-Type: image/png; name=\"image.png\"\r\n" +
		"Content-Disposition: attachment; filename=\"image.png\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		encoded + "\r\n" +
		"--boundary-bin-001--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != "See attached image." {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "See attached image.")
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}

	att := msg.Attachments[0]
	if att.Filename != "image.png" {
		t.Errorf("filename = %q, want %q", att.Filename, "image.png")
	}
	if att.ContentType != "image/png" {
		t.Errorf("content type = %q, want %q", att.ContentType, "image/png")
	}
	if !bytes.Equal(att.Content, binaryData) {
		t.Errorf("decoded content = %x, want %x", att.Content, binaryData)
	}
}

func TestParse_MultipartMixedWithMultipleAttachments(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Multiple Attachments\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary-multi-001\"\r\n" +
		"\r\n" +
		"--boundary-multi-001\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Body text.\r\n" +
		"--boundary-multi-001\r\n" +
		"Content-Type: application/pdf; name=\"report.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"report.pdf\"\r\n" +
		"\r\n" +
		"PDF-CONTENT-HERE\r\n" +
		"--boundary-multi-001\r\n" +
		"Content-Type: text/csv; name=\"data.csv\"\r\n" +
		"Content-Disposition: attachment; filename=\"data.csv\"\r\n" +
		"\r\n" +
		"col1,col2\r\nval1,val2\r\n" +
		"--boundary-multi-001--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != "Body text." {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "Body text.")
	}
	if len(msg.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(msg.Attachments))
	}

	if msg.Attachments[0].Filename != "report.pdf" {
		t.Errorf("first attachment filename = %q, want %q", msg.Attachments[0].Filename, "report.pdf")
	}
	if msg.Attachments[0].ContentType != "application/pdf" {
		t.Errorf("first attachment content type = %q, want %q", msg.Attachments[0].ContentType, "application/pdf")
	}
	if msg.Attachments[1].Filename != "data.csv" {
		t.Errorf("second attachment filename = %q, want %q", msg.Attachments[1].Filename, "data.csv")
	}
}

func TestParse_InlineImage(t *testing.T) {
	binaryData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG magic bytes
	encoded := base64.StdEncoding.EncodeToString(binaryData)

	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Inline Image\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/related; boundary=\"boundary-rel-001\"\r\n" +
		"\r\n" +
		"--boundary-rel-001\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body><img src=\"cid:img1\"/></body></html>\r\n" +
		"--boundary-rel-001\r\n" +
		"Content-Type: image/jpeg; name=\"photo.jpg\"\r\n" +
		"Content-Disposition: inline; filename=\"photo.jpg\"\r\n" +
		"Content-Id: <img1>\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		encoded + "\r\n" +
		"--boundary-rel-001--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantHTML := "<html><body><img src=\"cid:img1\"/></body></html>"
	if msg.HTMLBody != wantHTML {
		t.Errorf("HTMLBody = %q, want %q", msg.HTMLBody, wantHTML)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}

	att := msg.Attachments[0]
	if !att.IsInline {
		t.Error("attachment should be inline")
	}
	if att.ContentID != "img1" {
		t.Errorf("ContentID = %q, want %q", att.ContentID, "img1")
	}
	if att.Filename != "photo.jpg" {
		t.Errorf("filename = %q, want %q", att.Filename, "photo.jpg")
	}
	if !bytes.Equal(att.Content, binaryData) {
		t.Errorf("decoded content = %x, want %x", att.Content, binaryData)
	}
}

func TestParse_NestedMultipart(t *testing.T) {
	binaryData := []byte{0x00, 0x01, 0x02}
	encoded := base64.StdEncoding.EncodeToString(binaryData)

	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Nested Multipart\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"outer-boundary\"\r\n" +
		"\r\n" +
		"--outer-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=\"inner-boundary\"\r\n" +
		"\r\n" +
		"--inner-boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain nested text.\r\n" +
		"--inner-boundary\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>HTML nested text.</p>\r\n" +
		"--inner-boundary--\r\n" +
		"\r\n" +
		"--outer-boundary\r\n" +
		"Content-Type: application/octet-stream; name=\"data.bin\"\r\n" +
		"Content-Disposition: attachment; filename=\"data.bin\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		encoded + "\r\n" +
		"--outer-boundary--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != "Plain nested text." {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "Plain nested text.")
	}
	if msg.HTMLBody != "<p>HTML nested text.</p>" {
		t.Errorf("HTMLBody = %q, want %q", msg.HTMLBody, "<p>HTML nested text.</p>")
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}
	if msg.Attachments[0].Filename != "data.bin" {
		t.Errorf("attachment filename = %q, want %q", msg.Attachments[0].Filename, "data.bin")
	}
	if !bytes.Equal(msg.Attachments[0].Content, binaryData) {
		t.Errorf("attachment content = %x, want %x", msg.Attachments[0].Content, binaryData)
	}
}

func TestParse_NoContentTypeHeader(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: No Content-Type\r\n" +
		"\r\n" +
		"Default plain text body.\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != "Default plain text body.\r\n" {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, "Default plain text body.\r\n")
	}
	if msg.HTMLBody != "" {
		t.Errorf("HTMLBody should be empty, got %q", msg.HTMLBody)
	}
}

func TestParse_EmptyBody(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Empty\r\n" +
		"\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != "" {
		t.Errorf("TextBody should be empty, got %q", msg.TextBody)
	}
	if msg.HTMLBody != "" {
		t.Errorf("HTMLBody should be empty, got %q", msg.HTMLBody)
	}
	if msg.Subject != "Empty" {
		t.Errorf("subject = %q, want %q", msg.Subject, "Empty")
	}
}

func TestParse_QuotedPrintableEncoding(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: QP Test\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"This is a long line that has been soft-wrapped with quoted-printable =\r\n" +
		"encoding. Special chars: =C3=A9 (=C3=A9 is e-acute).\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Quoted-printable decodes soft line break (=\r\n) and =C3=A9 -> e-acute (U+00E9).
	if !strings.Contains(msg.TextBody, "soft-wrapped with quoted-printable encoding") {
		t.Errorf("TextBody should contain joined line, got %q", msg.TextBody)
	}
	if !strings.Contains(msg.TextBody, "\u00e9") {
		t.Errorf("TextBody should contain decoded e-acute character, got %q", msg.TextBody)
	}
}

func TestParse_Base64EncodedTextPart(t *testing.T) {
	plainText := "This text was base64 encoded in the email."
	encoded := base64.StdEncoding.EncodeToString([]byte(plainText))

	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Base64 Text\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		encoded + "\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != plainText {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, plainText)
	}
}

func TestParse_QuotedPrintableInMultipart(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: QP Multipart\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"qp-boundary\"\r\n" +
		"\r\n" +
		"--qp-boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Caf=C3=A9 au lait\r\n" +
		"--qp-boundary\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"<p>Caf=C3=A9 au lait</p>\r\n" +
		"--qp-boundary--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The multipart reader strips the trailing \r\n before the boundary.
	// The QP decoder then decodes the remaining content.
	wantText := "Caf\u00e9 au lait"
	if msg.TextBody != wantText {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, wantText)
	}

	wantHTML := "<p>Caf\u00e9 au lait</p>"
	if msg.HTMLBody != wantHTML {
		t.Errorf("HTMLBody = %q, want %q", msg.HTMLBody, wantHTML)
	}
}

func TestParse_Base64InMultipart(t *testing.T) {
	textContent := "Base64 text inside multipart"
	encoded := base64.StdEncoding.EncodeToString([]byte(textContent))

	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Base64 Multipart\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"b64-boundary\"\r\n" +
		"\r\n" +
		"--b64-boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		encoded + "\r\n" +
		"--b64-boundary--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.TextBody != textContent {
		t.Errorf("TextBody = %q, want %q", msg.TextBody, textContent)
	}
}

func TestParse_FilenameFromContentTypeName(t *testing.T) {
	// When Content-Disposition lacks a filename, fall back to Content-Type name param.
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Name Fallback\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"name-boundary\"\r\n" +
		"\r\n" +
		"--name-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Body.\r\n" +
		"--name-boundary\r\n" +
		"Content-Type: application/pdf; name=\"fallback.pdf\"\r\n" +
		"Content-Disposition: attachment\r\n" +
		"\r\n" +
		"PDF-DATA\r\n" +
		"--name-boundary--\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}
	if msg.Attachments[0].Filename != "fallback.pdf" {
		t.Errorf("filename = %q, want %q", msg.Attachments[0].Filename, "fallback.pdf")
	}
}

func TestParse_HeadersPreserved(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Cc: carol@example.com\r\n" +
		"Subject: Headers Test\r\n" +
		"X-Custom-Header: custom-value\r\n" +
		"\r\n" +
		"Body content.\r\n"

	msg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := msg.Headers.Get("From"); v != "alice@example.com" {
		t.Errorf("From header = %q, want %q", v, "alice@example.com")
	}
	if v := msg.Headers.Get("To"); v != "bob@example.com" {
		t.Errorf("To header = %q, want %q", v, "bob@example.com")
	}
	if v := msg.Headers.Get("X-Custom-Header"); v != "custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", v, "custom-value")
	}
}
