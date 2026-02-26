// Package mimeparse provides RFC 5322 MIME message parsing,
// extracting text/HTML bodies and attachments from raw email messages.
package mimeparse

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"mime/quotedprintable"
)

// ParsedMessage holds the structured parts extracted from a raw RFC 5322 message.
type ParsedMessage struct {
	Subject     string
	Headers     mail.Header
	TextBody    string
	HTMLBody    string
	Attachments []Attachment
}

// Attachment represents a single MIME attachment or inline part.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
	ContentID   string // for inline images (cid:xxx)
	IsInline    bool
}

// Parse parses a raw RFC 5322 message (headers + body) into structured parts.
// For non-multipart messages, the body is placed in TextBody or HTMLBody based on Content-Type.
// For multipart messages, it walks all parts recursively.
func Parse(raw []byte) (*ParsedMessage, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("mimeparse: failed to read message: %w", err)
	}

	parsed := &ParsedMessage{
		Headers: msg.Header,
		Subject: msg.Header.Get("Subject"),
	}

	contentType := msg.Header.Get("Content-Type")
	transferEncoding := msg.Header.Get("Content-Transfer-Encoding")

	if contentType == "" {
		// No Content-Type header; treat as text/plain per RFC 2045.
		body, err := readBody(msg.Body, transferEncoding)
		if err != nil {
			return nil, fmt.Errorf("mimeparse: failed to read body: %w", err)
		}
		parsed.TextBody = string(body)
		return parsed, nil
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("mimeparse: failed to parse Content-Type: %w", err)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return nil, fmt.Errorf("mimeparse: multipart message missing boundary")
		}
		if err := walkMultipart(msg.Body, boundary, parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}

	// Non-multipart single-part message.
	body, err := readBody(msg.Body, transferEncoding)
	if err != nil {
		return nil, fmt.Errorf("mimeparse: failed to read body: %w", err)
	}

	switch {
	case strings.HasPrefix(mediaType, "text/html"):
		parsed.HTMLBody = string(body)
	default:
		parsed.TextBody = string(body)
	}

	return parsed, nil
}

// walkMultipart recursively processes a multipart MIME body.
func walkMultipart(r io.Reader, boundary string, parsed *ParsedMessage) error {
	mr := multipart.NewReader(r, boundary)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mimeparse: failed to read next part: %w", err)
		}

		partCT := part.Header.Get("Content-Type")
		partTE := part.Header.Get("Content-Transfer-Encoding")

		mediaType := "text/plain"
		var params map[string]string
		if partCT != "" {
			var err error
			mediaType, params, err = mime.ParseMediaType(partCT)
			if err != nil {
				// If we cannot parse the Content-Type, treat as attachment.
				mediaType = "application/octet-stream"
			}
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			// Nested multipart -- recurse.
			nestedBoundary := params["boundary"]
			if nestedBoundary == "" {
				continue
			}
			if err := walkMultipart(part, nestedBoundary, parsed); err != nil {
				return err
			}
			continue
		}

		body, err := readBody(part, partTE)
		if err != nil {
			return fmt.Errorf("mimeparse: failed to read part body: %w", err)
		}

		switch {
		case mediaType == "text/plain" && parsed.TextBody == "":
			parsed.TextBody = string(body)
		case mediaType == "text/html" && parsed.HTMLBody == "":
			parsed.HTMLBody = string(body)
		default:
			att := buildAttachment(part, mediaType, params, body)
			parsed.Attachments = append(parsed.Attachments, att)
		}
	}
}

// buildAttachment constructs an Attachment from a MIME part.
func buildAttachment(part *multipart.Part, mediaType string, params map[string]string, body []byte) Attachment {
	disposition := part.Header.Get("Content-Disposition")

	filename := ""
	isInline := false

	if disposition != "" {
		dispType, dispParams, err := mime.ParseMediaType(disposition)
		if err == nil {
			filename = dispParams["filename"]
			isInline = strings.EqualFold(dispType, "inline")
		}
	}

	// Fallback: try "name" param from Content-Type.
	if filename == "" {
		filename = params["name"]
	}

	contentID := part.Header.Get("Content-Id")
	contentID = strings.TrimPrefix(contentID, "<")
	contentID = strings.TrimSuffix(contentID, ">")

	return Attachment{
		Filename:    filename,
		ContentType: mediaType,
		Content:     body,
		ContentID:   contentID,
		IsInline:    isInline,
	}
}

// readBody reads the full contents of r, decoding the Content-Transfer-Encoding
// (base64 or quoted-printable) when applicable.
func readBody(r io.Reader, transferEncoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(transferEncoding)) {
	case "base64":
		return io.ReadAll(base64.NewDecoder(base64.StdEncoding, r))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(r))
	default:
		// 7bit, 8bit, binary, or empty -- read directly.
		return io.ReadAll(r)
	}
}
