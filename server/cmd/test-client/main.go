// Package main provides a standalone CLI tool for sending test emails
// through the smtp-proxy SMTP server. It supports STARTTLS, implicit TLS,
// plaintext connections, SMTP AUTH PLAIN, and batch sending with rate limiting.
//
// Usage:
//
//	test-client --from sender@example.com --to recipient@example.com --subject "Test" --body "Hello"
//	test-client --tls starttls --insecure --count 10 --rate 5
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type config struct {
	host     string
	port     int
	tlsMode  string
	insecure bool
	user     string
	password string
	from     string
	to       stringSlice
	subject  string
	body     string
	count    int
	rate     float64
}

// stringSlice implements flag.Value for repeatable --to flags.
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	cfg := parseFlags()

	if cfg.from == "" {
		fmt.Fprintln(os.Stderr, "error: --from is required")
		flag.Usage()
		os.Exit(2)
	}
	if len(cfg.to) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one --to is required")
		flag.Usage()
		os.Exit(2)
	}

	addr := fmt.Sprintf("%s:%d", cfg.host, cfg.port)

	fmt.Printf("SMTP Test Client\n")
	fmt.Printf("  Server:   %s\n", addr)
	fmt.Printf("  TLS:      %s\n", cfg.tlsMode)
	fmt.Printf("  From:     %s\n", cfg.from)
	fmt.Printf("  To:       %s\n", strings.Join(cfg.to, ", "))
	fmt.Printf("  Count:    %d\n", cfg.count)
	if cfg.count > 1 {
		fmt.Printf("  Rate:     %.1f emails/sec\n", cfg.rate)
	}
	fmt.Println()

	var (
		successCount int
		failCount    int
		totalSend    time.Duration
	)

	interval := time.Duration(0)
	if cfg.count > 1 && cfg.rate > 0 {
		interval = time.Duration(float64(time.Second) / cfg.rate)
	}

	for i := 0; i < cfg.count; i++ {
		if i > 0 && interval > 0 {
			time.Sleep(interval)
		}

		seq := i + 1
		subject := cfg.subject
		body := cfg.body
		if cfg.count > 1 {
			subject = fmt.Sprintf("%s [%d/%d]", cfg.subject, seq, cfg.count)
			body = fmt.Sprintf("%s\n\n-- Email %d of %d --", cfg.body, seq, cfg.count)
		}

		sendStart := time.Now()
		err := sendEmail(cfg, addr, subject, body)
		sendDuration := time.Since(sendStart)
		totalSend += sendDuration

		if err != nil {
			failCount++
			fmt.Printf("  [%d/%d] FAIL (%s): %v\n", seq, cfg.count, sendDuration, err)
		} else {
			successCount++
			fmt.Printf("  [%d/%d] OK   (%s)\n", seq, cfg.count, sendDuration)
		}
	}

	fmt.Println()
	fmt.Printf("Results: %d sent, %d failed, total time %s\n", successCount, failCount, totalSend)

	if failCount > 0 {
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config

	flag.StringVar(&cfg.host, "host", "localhost", "SMTP server host")
	flag.IntVar(&cfg.port, "port", 587, "SMTP server port")
	flag.StringVar(&cfg.tlsMode, "tls", "starttls", "TLS mode: starttls, implicit, none")
	flag.BoolVar(&cfg.insecure, "insecure", false, "Skip TLS certificate verification")
	flag.StringVar(&cfg.user, "user", "", "SMTP AUTH username")
	flag.StringVar(&cfg.password, "password", "", "SMTP AUTH password")
	flag.StringVar(&cfg.from, "from", "", "Sender email address")
	flag.Var(&cfg.to, "to", "Recipient email address (can be specified multiple times)")
	flag.StringVar(&cfg.subject, "subject", "Test Email", "Email subject")
	flag.StringVar(&cfg.body, "body", "This is a test email sent by smtp-proxy test-client.", "Email body")
	flag.IntVar(&cfg.count, "count", 1, "Number of emails to send (for batch testing)")
	flag.Float64Var(&cfg.rate, "rate", 1, "Emails per second for batch sending")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: test-client [options]\n\n")
		fmt.Fprintf(os.Stderr, "A CLI tool for sending test emails through the smtp-proxy SMTP server.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  test-client --from test@example.com --to recipient@example.com\n")
		fmt.Fprintf(os.Stderr, "  test-client --tls none --from test@example.com --to recipient@example.com\n")
		fmt.Fprintf(os.Stderr, "  test-client --insecure --user admin --password secret --from test@example.com --to recipient@example.com\n")
		fmt.Fprintf(os.Stderr, "  test-client --count 100 --rate 10 --from test@example.com --to recipient@example.com\n")
	}

	flag.Parse()
	return cfg
}

func sendEmail(cfg config, addr, subject, body string) error {
	tlsConfig := &tls.Config{
		ServerName:         cfg.host,
		InsecureSkipVerify: cfg.insecure, //nolint:gosec // Intentional for dev self-signed certs.
	}

	msg := buildMessage(cfg.from, cfg.to, subject, body)

	switch cfg.tlsMode {
	case "none":
		return sendPlain(addr, cfg, msg)
	case "implicit":
		return sendImplicitTLS(addr, cfg, tlsConfig, msg)
	case "starttls":
		return sendSTARTTLS(addr, cfg, tlsConfig, msg)
	default:
		return fmt.Errorf("unknown TLS mode: %s (use starttls, implicit, or none)", cfg.tlsMode)
	}
}

func sendPlain(addr string, cfg config, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Close()

	return smtpSend(c, cfg, msg)
}

func sendSTARTTLS(addr string, cfg config, tlsConfig *tls.Config, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Close()

	if err := c.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starttls: %w", err)
	}

	return smtpSend(c, cfg, msg)
}

func sendImplicitTLS(addr string, cfg config, tlsConfig *tls.Config, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return fmt.Errorf("split host port: %w", err)
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("new client: %w", err)
	}
	defer c.Close()

	return smtpSend(c, cfg, msg)
}

func smtpSend(c *smtp.Client, cfg config, msg []byte) error {
	// Authenticate if credentials are provided.
	if cfg.user != "" && cfg.password != "" {
		auth := smtp.PlainAuth("", cfg.user, cfg.password, cfg.host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := c.Mail(cfg.from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}

	for _, rcpt := range cfg.to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt to %s: %w", rcpt, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return c.Quit()
}

func buildMessage(from string, to []string, subject, body string) []byte {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)

	return []byte(sb.String())
}
