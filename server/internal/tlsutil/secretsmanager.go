package tlsutil

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/rs/zerolog"
)

// SecretsManagerFetcher abstracts the Secrets Manager API call so tests can inject a mock
// and main.go can wire in the real client without importing this package's internals.
//
// @MX:ANCHOR: Interface boundary used by Provider (constructor), tests, and main.go.
// @MX:REASON: All real and mock fetchers satisfy this; changing the signature breaks all callers.
// @MX:SPEC: SPEC-TLS-001
type SecretsManagerFetcher interface {
	GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// secretEntry represents one domain entry in the Secrets Manager JSON value.
type secretEntry struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

// bundle is an immutable snapshot of all loaded certificates.
// It is replaced atomically on each successful reload; never mutated in place.
type bundle struct {
	// byDomain maps each domain name to its parsed TLS certificate.
	byDomain map[string]*tls.Certificate
	// defaultCert is the certificate served when a handshake carries no SNI.
	// Determined by DefaultCert config; falls back to the first sorted domain key.
	defaultCert *tls.Certificate
}

// Provider loads TLS certificates from AWS Secrets Manager and serves them via
// the GetCertificate callback, enabling hot-reload without server restart.
//
// @MX:ANCHOR: GetCertificate is called on every TLS handshake via tls.Config.
// @MX:REASON: High fan-in from the TLS stack; the atomic.Pointer load MUST remain lock-free.
// @MX:SPEC: SPEC-TLS-001
type Provider struct {
	fetcher        SecretsManagerFetcher
	secretID       string
	defaultDomain  string
	reloadInterval time.Duration
	selfSigned     tls.Certificate
	log            zerolog.Logger
	bundle         atomic.Pointer[bundle]
}

const defaultReloadHours = 168

// NewProvider creates a Provider that loads certificates from Secrets Manager.
//
// Parameters:
//   - fetcher: Secrets Manager client (nil is allowed; disables remote loading).
//   - secretID: Secrets Manager secret ID or ARN. Empty string disables SM loading.
//   - defaultDomain: domain key for no-SNI handshakes. Empty means sorted-first.
//   - reloadInterval: how often to reload. Values <= 0 fall back to defaultReloadHours.
//   - selfSigned: fallback certificate, generated once at startup.
//   - log: structured logger.
func NewProvider(
	fetcher SecretsManagerFetcher,
	secretID string,
	defaultDomain string,
	reloadInterval time.Duration,
	selfSigned tls.Certificate,
	log zerolog.Logger,
) *Provider {
	if reloadInterval <= 0 {
		reloadInterval = time.Duration(defaultReloadHours) * time.Hour
	}
	return &Provider{
		fetcher:        fetcher,
		secretID:       secretID,
		defaultDomain:  defaultDomain,
		reloadInterval: reloadInterval,
		selfSigned:     selfSigned,
		log:            log,
	}
}

// Start performs the initial synchronous load and then runs the periodic reload
// loop in a background goroutine bound to ctx.
//
// On startup failure the provider falls back to self-signed (N1/AC6).
// The goroutine stops when ctx is cancelled, supporting graceful shutdown.
//
// @MX:WARN: Background goroutine with a ticker; must be bound to ctx to prevent leaks.
// @MX:REASON: The reload loop runs for the server lifetime; an unbound goroutine would leak on shutdown.
// @MX:SPEC: SPEC-TLS-001
func (p *Provider) Start(ctx context.Context) {
	if p.fetcher == nil || p.secretID == "" {
		p.log.Info().Msg("TLS provider: Secrets Manager not configured; using self-signed certificate")
		return
	}

	// Synchronous initial load so the provider is ready before accepting connections.
	if err := p.load(ctx); err != nil {
		p.log.Warn().Err(err).Msg("TLS provider: initial Secrets Manager load failed; falling back to self-signed")
	}

	go func() {
		ticker := time.NewTicker(p.reloadInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				p.log.Debug().Msg("TLS provider: reload goroutine stopping")
				return
			case <-ticker.C:
				if err := p.load(ctx); err != nil {
					// N3/AC8: retain last-good bundle; log and retry next tick.
					p.log.Warn().Err(err).Msg("TLS provider: reload failed; retaining last-good certificate set")
				}
			}
		}
	}()
}

// GetCertificate implements tls.Config.GetCertificate.
// It is called on every TLS handshake and MUST be lock-free.
//
// Selection logic:
//   - SNI present and matches a loaded domain → that domain's certificate.
//   - SNI present but no match → self-signed (E2, E3, AC4).
//   - No SNI → bundle default certificate, or self-signed if no bundle (E4, AC5).
//
// @MX:ANCHOR: Called on every TLS handshake; lock-free by design (atomic.Pointer.Load).
// @MX:REASON: tls.Config wires this directly; any lock would serialize all SMTP connections.
// @MX:SPEC: SPEC-TLS-001
func (p *Provider) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	b := p.bundle.Load()

	if hello.ServerName != "" {
		if b != nil {
			if cert, ok := b.byDomain[hello.ServerName]; ok {
				return cert, nil
			}
		}
		// SNI present but no matching domain → self-signed (E3/AC4).
		return &p.selfSigned, nil
	}

	// No SNI: serve the default certificate from the bundle, or self-signed.
	if b != nil && b.defaultCert != nil {
		return b.defaultCert, nil
	}
	return &p.selfSigned, nil
}

// load fetches the secret, parses it, and atomically replaces the active bundle.
// Per-entry errors are logged and skipped (N2/AC10).
// If the new bundle would be empty after skipping bad entries, the old bundle is
// retained and an error is returned.
func (p *Provider) load(ctx context.Context) error {
	out, err := p.fetcher.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(p.secretID),
	})
	if err != nil {
		return fmt.Errorf("fetch secret %q: %w", p.secretID, err)
	}

	if out.SecretString == nil {
		return fmt.Errorf("secret %q has no string value", p.secretID)
	}

	entries := make(map[string]secretEntry)
	if err := json.Unmarshal([]byte(*out.SecretString), &entries); err != nil {
		return fmt.Errorf("parse secret JSON for %q: %w", p.secretID, err)
	}

	byDomain := make(map[string]*tls.Certificate, len(entries))
	for domain, entry := range entries {
		cert, err := tls.X509KeyPair([]byte(entry.Cert), []byte(entry.Key))
		if err != nil {
			// N2/AC10: skip invalid entry, log, continue.
			p.log.Warn().Err(err).Str("domain", domain).Msg("TLS provider: skipping invalid cert/key pair")
			continue
		}
		c := cert // local copy for pointer safety
		byDomain[domain] = &c
	}

	if len(byDomain) == 0 {
		return fmt.Errorf("secret %q contained no valid certificate entries", p.secretID)
	}

	// Determine the default certificate.
	// Use the configured domain if present; otherwise the first key in sorted order (AC5).
	var defaultCert *tls.Certificate
	if p.defaultDomain != "" {
		if cert, ok := byDomain[p.defaultDomain]; ok {
			defaultCert = cert
		}
	}
	if defaultCert == nil {
		// Fall back to sorted-first domain for determinism (AC5).
		keys := make([]string, 0, len(byDomain))
		for k := range byDomain {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		defaultCert = byDomain[keys[0]]
	}

	p.bundle.Store(&bundle{
		byDomain:    byDomain,
		defaultCert: defaultCert,
	})
	p.log.Info().Int("domains", len(byDomain)).Msg("TLS provider: certificate bundle loaded")
	return nil
}
