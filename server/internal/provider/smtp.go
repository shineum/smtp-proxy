package provider

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SMTP implements the Provider interface by relaying messages to an upstream
// SMTP server. It supports plain, STARTTLS, and implicit TLS transport, and
// optional PLAIN auth when Username and Password are provided.
type SMTP struct {
	host          string
	port          int
	username      string
	password      string
	encryption    string
	timeout       time.Duration
	defaultSender string

	// dial is overridable for tests; when nil the default net.Dialer is used.
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
	// tlsConfig is overridable for tests; when nil a server-name-only config is built.
	tlsConfig *tls.Config
}

// NewSMTP creates an SMTP provider from the given configuration.
func NewSMTP(cfg ProviderConfig) *SMTP {
	port := cfg.Port
	if port == 0 {
		port = defaultSMTPPort
	}
	enc := cfg.Encryption
	if enc == "" {
		enc = defaultSMTPEncryption
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &SMTP{
		host:          cfg.Host,
		port:          port,
		username:      cfg.Username,
		password:      cfg.Password,
		encryption:    enc,
		timeout:       timeout,
		defaultSender: cfg.DefaultSender,
	}
}

// GetName returns the provider identifier.
func (s *SMTP) GetName() string { return "smtp" }

// addr returns the host:port used to dial the upstream SMTP server.
func (s *SMTP) addr() string {
	return net.JoinHostPort(s.host, strconv.Itoa(s.port))
}

// @MX:ANCHOR: SMTP relay send path — invariant contract for outbound SMTP delivery.
// @MX:REASON: Called by factory.NewProvider via the Provider interface (fan_in >= 3:
// delivery worker, test-email handler, fallback chain); BCC must stay out of rendered
// headers, default_sender must override msg.From, and reply code classification must
// match RFC 5321 5xx=permanent / 4xx=transient.
//
// Send delivers a message via SMTP. The message is rendered as RFC 5322
// multipart/mixed using buildRawMIME (shared with the SES provider) and then
// transmitted with MAIL FROM / RCPT TO / DATA. CC and BCC addresses are
// included in the RCPT TO envelope; BCC is intentionally not added to the
// rendered headers.
func (s *SMTP) Send(ctx context.Context, msg *Message) (*DeliveryResult, error) {
	if s.host == "" {
		return nil, errors.New("smtp: host is not configured")
	}

	from := msg.From
	if s.defaultSender != "" {
		from = s.defaultSender
	}
	if from == "" {
		return nil, errors.New("smtp: empty MAIL FROM (no sender on message and no default_sender configured)")
	}

	recipients := allRecipients(msg)
	if len(recipients) == 0 {
		return nil, errors.New("smtp: no recipients (To/Cc/Bcc all empty)")
	}

	// Render the MIME body. We reuse buildRawMIME so that the on-the-wire
	// shape matches what SES sees when it forwards raw messages.
	renderMsg := *msg
	renderMsg.From = from
	body, err := buildRawMIME(&renderMsg)
	if err != nil {
		return nil, fmt.Errorf("smtp: build mime: %w", err)
	}

	client, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if s.username != "" && s.password != "" {
		auth, err := s.pickAuth(client)
		if err != nil {
			return nil, fmt.Errorf("smtp: auth: %w", err)
		}
		if err := client.Auth(auth); err != nil {
			return nil, fmt.Errorf("smtp: auth: %w", classifySMTPError(err))
		}
	}

	if err := client.Mail(from); err != nil {
		return nil, fmt.Errorf("smtp: MAIL FROM %s: %w", from, classifySMTPError(err))
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return nil, fmt.Errorf("smtp: RCPT TO %s: %w", rcpt, classifySMTPError(err))
		}
	}

	w, err := client.Data()
	if err != nil {
		return nil, fmt.Errorf("smtp: DATA: %w", classifySMTPError(err))
	}
	if _, err := w.Write(body); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("smtp: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("smtp: close data: %w", classifySMTPError(err))
	}

	// QUIT — best-effort; if it fails the message is already accepted.
	_ = client.Quit()

	return &DeliveryResult{
		ProviderMessageID: "smtp-" + msg.ID,
		Status:            StatusSent,
		Timestamp:         time.Now(),
		Metadata: map[string]string{
			"host":       s.host,
			"port":       strconv.Itoa(s.port),
			"encryption": s.encryption,
		},
	}, nil
}

// HealthCheck verifies the SMTP server is reachable and willing to negotiate
// the configured encryption mode. It does not perform AUTH because credentials
// may be wrong without the server being unhealthy.
func (s *SMTP) HealthCheck(ctx context.Context) error {
	if s.host == "" {
		return errors.New("smtp: host is not configured")
	}
	client, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Noop(); err != nil {
		return fmt.Errorf("smtp: NOOP: %w", err)
	}
	return client.Quit()
}

// connect dials the upstream SMTP server using the configured encryption mode
// and returns a ready-to-use smtp.Client. The returned client is wired with
// context cancellation: when ctx is canceled, the underlying connection is
// closed which causes any in-flight SMTP command to return an error.
func (s *SMTP) connect(ctx context.Context) (*smtp.Client, error) {
	addr := s.addr()

	dial := s.dial
	if dial == nil {
		dialer := &net.Dialer{Timeout: s.timeout}
		dial = dialer.DialContext
	}

	tlsCfg := s.tlsConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
	}

	var (
		conn net.Conn
		err  error
	)

	switch s.encryption {
	case SMTPEncryptionTLS:
		// Implicit TLS: wrap the raw connection in TLS immediately.
		rawConn, derr := dial(ctx, "tcp", addr)
		if derr != nil {
			return nil, fmt.Errorf("smtp: dial %s: %w", addr, derr)
		}
		tlsConn := tls.Client(rawConn, tlsCfg)
		if herr := tlsConn.HandshakeContext(ctx); herr != nil {
			_ = rawConn.Close()
			return nil, fmt.Errorf("smtp: tls handshake %s: %w", addr, herr)
		}
		conn = tlsConn
	default:
		// Plain or STARTTLS: start with an unencrypted TCP connection.
		conn, err = dial(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("smtp: dial %s: %w", addr, err)
		}
	}

	// Tie connection lifetime to ctx so a cancelled context aborts I/O.
	go func() {
		<-ctx.Done()
		_ = conn.SetDeadline(time.Now())
	}()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp: new client %s: %w", addr, err)
	}

	if s.encryption == SMTPEncryptionStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, fmt.Errorf("smtp: server %s does not advertise STARTTLS", addr)
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("smtp: starttls %s: %w", addr, err)
		}
	}

	return client, nil
}

// allRecipients returns the union of To, Cc, and Bcc for the SMTP envelope.
func allRecipients(msg *Message) []string {
	out := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	out = append(out, msg.To...)
	out = append(out, msg.CC...)
	out = append(out, msg.BCC...)
	return out
}

// classifySMTPError wraps an SMTP server error as a ProviderError with
// permanent/transient classification derived from the 3-digit reply code.
// Per RFC 5321: 5xx codes are permanent failures; 4xx are transient.
// net/smtp surfaces server replies wrapped in *textproto.Error.
func classifySMTPError(err error) error {
	if err == nil {
		return nil
	}
	var serr *textproto.Error
	if errors.As(err, &serr) {
		return &ProviderError{
			Provider:   "smtp",
			StatusCode: serr.Code,
			Message:    serr.Msg,
			Permanent:  serr.Code >= 500 && serr.Code < 600,
		}
	}
	// Best-effort parse for plain-string error format ("NNN message").
	if code, msg, ok := parseSMTPReplyCode(err.Error()); ok {
		return &ProviderError{
			Provider:   "smtp",
			StatusCode: code,
			Message:    msg,
			Permanent:  code >= 500 && code < 600,
		}
	}
	// Unknown shape — treat as transient so callers retry.
	return &ProviderError{
		Provider:  "smtp",
		Message:   err.Error(),
		Permanent: false,
	}
}

// @MX:NOTE: SASL mechanism selection — PLAIN preferred, LOGIN fallback.
// Some legacy Exchange/O365 tenants and Korean ISP / on-prem relays advertise
// only AUTH=LOGIN. CRAM-MD5 and XOAUTH2 are intentionally unsupported here
// because they require shared-secret challenge or token plumbing that this
// provider does not own. Passwords containing shell-special characters
// (e.g. ! @ # $ % ^ & * " ' ?) are transported safely because net/smtp
// base64-encodes both PLAIN and LOGIN payloads before sending.
func (s *SMTP) pickAuth(client *smtp.Client) (smtp.Auth, error) {
	ok, mechsRaw := client.Extension("AUTH")
	if !ok {
		return nil, &ProviderError{
			Provider:  "smtp",
			Message:   "server does not advertise AUTH (credentials configured but no AUTH extension)",
			Permanent: true,
		}
	}
	mechs := parseAuthMechs(mechsRaw)
	if _, ok := mechs["PLAIN"]; ok {
		return smtp.PlainAuth("", s.username, s.password, s.host), nil
	}
	if _, ok := mechs["LOGIN"]; ok {
		return &loginAuth{username: s.username, password: s.password, host: s.host}, nil
	}
	return nil, &ProviderError{
		Provider:  "smtp",
		Message:   fmt.Sprintf("no supported AUTH mechanism advertised (got %q, want PLAIN or LOGIN)", mechsRaw),
		Permanent: true,
	}
}

// parseAuthMechs tokenizes a space-separated AUTH mechanism list (as returned
// by smtp.Client.Extension("AUTH")) into an uppercase set for membership tests.
func parseAuthMechs(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, tok := range strings.Fields(s) {
		if tok == "" {
			continue
		}
		out[strings.ToUpper(tok)] = struct{}{}
	}
	return out
}

// isLocalhostName mirrors net/smtp's internal isLocalhost check so that
// loginAuth can permit cleartext credentials on loopback connections only.
func isLocalhostName(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

// loginAuth implements the obsolete LOGIN SASL mechanism for net/smtp.Client.
// It is used only when the server advertises LOGIN but not PLAIN.
//
// Protocol exchange (after AUTH LOGIN):
//
//	server -> 334 base64("Username:")  (or vendor variant)
//	client -> base64(username)
//	server -> 334 base64("Password:")
//	client -> base64(password)
//
// Because real servers vary the challenge string ("Username:", "User Name",
// "username", etc.), we ignore the challenge text and emit credentials by
// step counter. Any challenge after step 1 is treated as a protocol error.
type loginAuth struct {
	username string
	password string
	host     string
	step     int
}

// Start refuses to send credentials over an unencrypted, non-loopback link.
// This mirrors net/smtp.PlainAuth's safety check; we deliberately skip the
// server-name-vs-host comparison because pickAuth runs after STARTTLS /
// implicit-TLS negotiation and the user has configured the host explicitly.
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhostName(server.Name) {
		return "", nil, errors.New("smtp: refusing LOGIN auth over unencrypted, non-loopback connection")
	}
	return "LOGIN", nil, nil
}

// Next emits username on the first server challenge and password on the second.
// fromServer is the base64-decoded challenge (already decoded by net/smtp).
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch a.step {
	case 0:
		a.step++
		return []byte(a.username), nil
	case 1:
		a.step++
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("smtp: unexpected server challenge after LOGIN credentials: %q", fromServer)
	}
}

// parseSMTPReplyCode parses leading "NNN " from an error string, returning the
// integer code and remaining message. Returns ok=false when no code is found.
func parseSMTPReplyCode(s string) (int, string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 4 || s[3] != ' ' {
		return 0, "", false
	}
	code, err := strconv.Atoi(s[:3])
	if err != nil || code < 200 || code > 599 {
		return 0, "", false
	}
	return code, s[4:], true
}
