package provider

import (
	"context"
	"errors"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

// captureBackend implements gosmtp.Backend, recording every accepted message.
type captureBackend struct {
	mu        sync.Mutex
	sessions  []*captureSession
	authPlain func(username, password string) error
	mailErr   *gosmtp.SMTPError // returned from Mail when non-nil
	rcptErr   *gosmtp.SMTPError // returned from Rcpt when non-nil
	dataErr   *gosmtp.SMTPError // returned from Data when non-nil
}

func (b *captureBackend) NewSession(_ *gosmtp.Conn) (gosmtp.Session, error) {
	s := &captureSession{backend: b}
	b.mu.Lock()
	b.sessions = append(b.sessions, s)
	b.mu.Unlock()
	return s, nil
}

func (b *captureBackend) lastSession() *captureSession {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.sessions) == 0 {
		return nil
	}
	return b.sessions[len(b.sessions)-1]
}

type captureSession struct {
	backend *captureBackend
	from    string
	rcpts   []string
	body    []byte
	authed  bool
}

// AuthMechanisms advertises PLAIN to support SASL PLAIN clients.
func (s *captureSession) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth wires SASL PLAIN to the backend's authPlain hook (when set).
func (s *captureSession) Auth(mech string) (sasl.Server, error) {
	if mech != sasl.Plain {
		return nil, errors.New("unsupported mechanism: " + mech)
	}
	return sasl.NewPlainServer(func(_, username, password string) error {
		if s.backend.authPlain != nil {
			if err := s.backend.authPlain(username, password); err != nil {
				return err
			}
		}
		s.authed = true
		return nil
	}), nil
}

func (s *captureSession) Mail(from string, _ *gosmtp.MailOptions) error {
	if s.backend.mailErr != nil {
		return s.backend.mailErr
	}
	s.from = from
	return nil
}

func (s *captureSession) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	if s.backend.rcptErr != nil {
		return s.backend.rcptErr
	}
	s.rcpts = append(s.rcpts, to)
	return nil
}

func (s *captureSession) Data(r io.Reader) error {
	if s.backend.dataErr != nil {
		// Drain to keep the protocol happy before returning the error.
		_, _ = io.Copy(io.Discard, r)
		return s.backend.dataErr
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.body = body
	return nil
}

func (s *captureSession) Reset()        {}
func (s *captureSession) Logout() error { return nil }

// startTestSMTPServer boots a gosmtp server on 127.0.0.1:0 (plain transport,
// PLAIN auth allowed) and returns the listening port and a stop function.
func startTestSMTPServer(t *testing.T, backend *captureBackend) (port int, stop func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port = listener.Addr().(*net.TCPAddr).Port

	server := gosmtp.NewServer(backend)
	server.Domain = "test.local"
	server.AllowInsecureAuth = true
	server.ReadTimeout = 5 * time.Second
	server.WriteTimeout = 5 * time.Second
	server.MaxMessageBytes = 1024 * 1024
	server.MaxRecipients = 50

	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()

	stop = func() {
		_ = server.Close()
		<-done
	}
	return port, stop
}

func TestSMTP_Send_PlainNoAuth(t *testing.T) {
	backend := &captureBackend{}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Encryption: SMTPEncryptionNone,
		Timeout:    3 * time.Second,
	})

	msg := &Message{
		ID:       "msg-1",
		From:     "sender@example.com",
		To:       []string{"to@example.com"},
		CC:       []string{"cc@example.com"},
		BCC:      []string{"bcc@example.com"},
		Subject:  "Hello SMTP",
		TextBody: "plain body",
		HTMLBody: "<p>html body</p>",
	}

	result, err := p.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.Status != StatusSent {
		t.Errorf("Status = %q, want %q", result.Status, StatusSent)
	}
	if result.ProviderMessageID != "smtp-msg-1" {
		t.Errorf("ProviderMessageID = %q, want smtp-msg-1", result.ProviderMessageID)
	}

	sess := backend.lastSession()
	if sess == nil {
		t.Fatal("server received no session")
	}
	if sess.from != "sender@example.com" {
		t.Errorf("MAIL FROM = %q, want sender@example.com", sess.from)
	}
	wantRcpts := []string{"to@example.com", "cc@example.com", "bcc@example.com"}
	if !equalStrings(sess.rcpts, wantRcpts) {
		t.Errorf("RCPT TO envelope = %v, want %v", sess.rcpts, wantRcpts)
	}
	bodyStr := string(sess.body)
	if !strings.Contains(bodyStr, "Subject: Hello SMTP") {
		t.Errorf("message body missing Subject header: %q", bodyStr)
	}
	if !strings.Contains(bodyStr, "plain body") {
		t.Errorf("message body missing text part: %q", bodyStr)
	}
	if !strings.Contains(bodyStr, "<p>html body</p>") {
		t.Errorf("message body missing html part: %q", bodyStr)
	}
	// BCC must NOT appear in rendered headers (only in envelope).
	if strings.Contains(bodyStr, "bcc@example.com") {
		t.Errorf("BCC address leaked into rendered headers: %q", bodyStr)
	}
}

func TestSMTP_Send_DefaultSenderOverride(t *testing.T) {
	backend := &captureBackend{}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:          "smtp",
		Host:          "127.0.0.1",
		Port:          port,
		Encryption:    SMTPEncryptionNone,
		DefaultSender: "verified@example.com",
		Timeout:       3 * time.Second,
	})

	msg := &Message{
		ID:       "msg-2",
		From:     "wrong@example.com",
		To:       []string{"to@example.com"},
		Subject:  "x",
		TextBody: "body",
	}

	if _, err := p.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	sess := backend.lastSession()
	if sess.from != "verified@example.com" {
		t.Errorf("MAIL FROM = %q, want verified@example.com (default_sender override)", sess.from)
	}
	if !strings.Contains(string(sess.body), "From: verified@example.com") {
		t.Errorf("rendered From header should use default_sender, body = %q", string(sess.body))
	}
}

func TestSMTP_Send_PlainAuthSuccess(t *testing.T) {
	var seenUser, seenPass string
	backend := &captureBackend{
		authPlain: func(u, p string) error {
			seenUser, seenPass = u, p
			return nil
		},
	}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Username:   "user1",
		Password:   "pass1",
		Encryption: SMTPEncryptionNone,
		Timeout:    3 * time.Second,
	})

	msg := &Message{
		ID:       "msg-3",
		From:     "from@example.com",
		To:       []string{"to@example.com"},
		Subject:  "s",
		TextBody: "body",
	}

	if _, err := p.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if seenUser != "user1" || seenPass != "pass1" {
		t.Errorf("AUTH credentials = (%q,%q), want (user1,pass1)", seenUser, seenPass)
	}
	if !backend.lastSession().authed {
		t.Error("session reports unauthenticated despite AUTH PLAIN call")
	}
}

func TestSMTP_Send_AuthFailureIsPermanent(t *testing.T) {
	backend := &captureBackend{
		authPlain: func(_, _ string) error {
			return &gosmtp.SMTPError{Code: 535, EnhancedCode: gosmtp.EnhancedCode{5, 7, 8}, Message: "auth bad"}
		},
	}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Username:   "u",
		Password:   "bad",
		Encryption: SMTPEncryptionNone,
		Timeout:    3 * time.Second,
	})

	_, err := p.Send(context.Background(), &Message{
		ID:       "msg-4",
		From:     "f@example.com",
		To:       []string{"t@example.com"},
		TextBody: "x",
	})
	if err == nil {
		t.Fatal("expected auth failure error, got nil")
	}
	if !IsPermanent(err) {
		t.Errorf("expected permanent error for 535 AUTH failure, got %v", err)
	}
}

func TestSMTP_Send_PermanentMailErrorReturned(t *testing.T) {
	backend := &captureBackend{
		mailErr: &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 1, 8}, Message: "sender rejected"},
	}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Encryption: SMTPEncryptionNone,
		Timeout:    3 * time.Second,
	})

	_, err := p.Send(context.Background(), &Message{
		ID:       "msg-5",
		From:     "spoof@example.com",
		To:       []string{"t@example.com"},
		TextBody: "x",
	})
	if err == nil {
		t.Fatal("expected MAIL FROM rejection, got nil")
	}
	if !IsPermanent(err) {
		t.Errorf("expected permanent error for 550, got %v", err)
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected error to wrap *ProviderError, got %T", err)
	}
	if pe.StatusCode != 550 {
		t.Errorf("StatusCode = %d, want 550", pe.StatusCode)
	}
}

func TestSMTP_Send_TransientRcptError(t *testing.T) {
	backend := &captureBackend{
		rcptErr: &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "try again later"},
	}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Encryption: SMTPEncryptionNone,
		Timeout:    3 * time.Second,
	})

	_, err := p.Send(context.Background(), &Message{
		ID:       "msg-6",
		From:     "f@example.com",
		To:       []string{"t@example.com"},
		TextBody: "x",
	})
	if err == nil {
		t.Fatal("expected RCPT TO error, got nil")
	}
	if IsPermanent(err) {
		t.Errorf("expected transient error for 451, got %v", err)
	}
}

func TestSMTP_Send_NoRecipients(t *testing.T) {
	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       2525, // unused; we never connect
		Encryption: SMTPEncryptionNone,
		Timeout:    1 * time.Second,
	})
	_, err := p.Send(context.Background(), &Message{
		ID:   "msg-7",
		From: "f@example.com",
	})
	if err == nil || !strings.Contains(err.Error(), "no recipients") {
		t.Errorf("expected no-recipients error, got %v", err)
	}
}

func TestSMTP_Send_EmptyFromNoDefault(t *testing.T) {
	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       2525,
		Encryption: SMTPEncryptionNone,
		Timeout:    1 * time.Second,
	})
	_, err := p.Send(context.Background(), &Message{
		ID: "msg-8",
		To: []string{"t@example.com"},
	})
	if err == nil || !strings.Contains(err.Error(), "MAIL FROM") {
		t.Errorf("expected empty-from error, got %v", err)
	}
}

func TestSMTP_Send_DialFailureWrapsError(t *testing.T) {
	// Listen and immediately close to obtain a guaranteed-closed port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Encryption: SMTPEncryptionNone,
		Timeout:    1 * time.Second,
	})
	_, err = p.Send(context.Background(), &Message{
		ID:       "msg-9",
		From:     "f@example.com",
		To:       []string{"t@example.com"},
		TextBody: "x",
	})
	if err == nil {
		t.Fatal("expected dial failure, got nil error")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Errorf("expected error message to mention dial, got %v", err)
	}
}

func TestSMTP_HealthCheck_Success(t *testing.T) {
	backend := &captureBackend{}
	port, stop := startTestSMTPServer(t, backend)
	defer stop()

	p := NewSMTP(ProviderConfig{
		Type:       "smtp",
		Host:       "127.0.0.1",
		Port:       port,
		Encryption: SMTPEncryptionNone,
		Timeout:    3 * time.Second,
	})
	if err := p.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck() = %v, want nil", err)
	}
}

func TestSMTP_HealthCheck_NoHost(t *testing.T) {
	p := NewSMTP(ProviderConfig{Type: "smtp"})
	if err := p.HealthCheck(context.Background()); err == nil {
		t.Error("expected error for empty host, got nil")
	}
}

func TestSMTP_GetName(t *testing.T) {
	p := NewSMTP(ProviderConfig{Type: "smtp", Host: "x", Port: 25})
	if name := p.GetName(); name != "smtp" {
		t.Errorf("GetName() = %q, want smtp", name)
	}
}

func TestClassifySMTPError_TextprotoError(t *testing.T) {
	err := &textproto.Error{Code: 550, Msg: "mailbox unavailable"}
	classified := classifySMTPError(err)
	var pe *ProviderError
	if !errors.As(classified, &pe) {
		t.Fatalf("expected *ProviderError, got %T", classified)
	}
	if pe.StatusCode != 550 || !pe.Permanent {
		t.Errorf("classifySMTPError(550) = (%d, perm=%v), want (550, true)", pe.StatusCode, pe.Permanent)
	}
}

func TestClassifySMTPError_TransientCode(t *testing.T) {
	err := &textproto.Error{Code: 421, Msg: "service not available"}
	classified := classifySMTPError(err)
	if IsPermanent(classified) {
		t.Errorf("4xx should be transient, got permanent: %v", classified)
	}
}

func TestClassifySMTPError_ParseFromString(t *testing.T) {
	classified := classifySMTPError(errors.New("502 command not implemented"))
	var pe *ProviderError
	if !errors.As(classified, &pe) {
		t.Fatalf("expected *ProviderError, got %T", classified)
	}
	if pe.StatusCode != 502 || !pe.Permanent {
		t.Errorf("classifySMTPError parsed = (%d, perm=%v), want (502, true)", pe.StatusCode, pe.Permanent)
	}
}

func TestClassifySMTPError_UnknownIsTransient(t *testing.T) {
	classified := classifySMTPError(errors.New("connection reset by peer"))
	if IsPermanent(classified) {
		t.Errorf("unknown errors should be transient, got permanent: %v", classified)
	}
}

func TestClassifySMTPError_NilPassthrough(t *testing.T) {
	if classifySMTPError(nil) != nil {
		t.Error("classifySMTPError(nil) should return nil")
	}
}

func TestParseSMTPReplyCode(t *testing.T) {
	tests := []struct {
		in       string
		wantCode int
		wantMsg  string
		wantOK   bool
	}{
		{"550 mailbox not found", 550, "mailbox not found", true},
		{"  421 try again later  ", 421, "try again later", true},
		{"hello world", 0, "", false},
		{"99 too small", 0, "", false},
		{"600 too large", 0, "", false},
		{"550", 0, "", false}, // missing trailing space + message
	}
	for _, tt := range tests {
		code, msg, ok := parseSMTPReplyCode(tt.in)
		if code != tt.wantCode || msg != tt.wantMsg || ok != tt.wantOK {
			t.Errorf("parseSMTPReplyCode(%q) = (%d,%q,%v), want (%d,%q,%v)",
				tt.in, code, msg, ok, tt.wantCode, tt.wantMsg, tt.wantOK)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
