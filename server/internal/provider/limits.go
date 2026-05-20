package provider

// MaxMessageBytes returns the maximum *total* on-the-wire MIME message size
// (headers + bodies + attachments) accepted by the named provider. A return
// value of 0 means "no known limit" — callers should treat the provider as
// unconstrained at this layer and let the network/upstream surface errors.
//
// Limits are conservative published values; ESPs sometimes accept slightly
// larger messages but document these as the hard caps:
//
//	sendgrid : 30 MB  (https://docs.sendgrid.com/api-reference/mail-send/limitations)
//	ses      : 40 MB  (raw email; v2 SendEmail SimpleContent is lower but we
//	                   use Raw mode whenever attachments are present)
//	mailgun  : 25 MB  (https://documentation.mailgun.com/en/latest/api-sending.html)
//	msgraph  :  4 MB  (single sendMail call; larger requires upload session)
//	smtp     : 25 MB  (operator default; the upstream relay may enforce its own)
//	stdout   : 25 MB  (artificial, to mirror SMTP behaviour in dev)
//	file     :  0     (no limit; this is a debug provider)
//
// REQ-MIME-006: pre-flight check so oversized messages fail with a clear
// "attachments exceed provider limit" error instead of being dispatched and
// rejected by the ESP after the network round-trip.
func MaxMessageBytes(providerName string) int64 {
	switch providerName {
	case "sendgrid":
		return 30 * 1024 * 1024
	case "ses":
		return 40 * 1024 * 1024
	case "mailgun":
		return 25 * 1024 * 1024
	case "msgraph":
		return 4 * 1024 * 1024
	case "smtp":
		return 25 * 1024 * 1024
	case "stdout":
		return 25 * 1024 * 1024
	default:
		return 0
	}
}

// EstimateMessageBytes returns an approximate on-the-wire size for the given
// message. Body text/HTML are counted at their raw byte length; attachments
// are counted at base64-encoded size (4/3 * raw, rounded up). The estimate
// intentionally includes a small fixed overhead for MIME headers and
// boundaries so callers can compare against MaxMessageBytes without
// reconstructing the full multipart wrapper.
func EstimateMessageBytes(msg *Message) int64 {
	if msg == nil {
		return 0
	}
	var n int64
	n += int64(len(msg.Body))
	n += int64(len(msg.TextBody))
	n += int64(len(msg.HTMLBody))
	n += int64(len(msg.Subject))
	for _, att := range msg.Attachments {
		// base64 encodes 3 bytes as 4 chars (rounded up).
		n += ((int64(len(att.Content)) + 2) / 3) * 4
		n += int64(len(att.Filename)) + int64(len(att.ContentType)) + 128 // MIME part headers
	}
	n += 512 // top-level headers + boundaries slack
	return n
}
