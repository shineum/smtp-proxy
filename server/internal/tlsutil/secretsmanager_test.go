package tlsutil

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/rs/zerolog"
)

// mockFetcher implements SecretsManagerFetcher for tests.
type mockFetcher struct {
	mu      sync.Mutex
	calls   int
	results []fetchResult
}

type fetchResult struct {
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func newMockFetcher(results ...fetchResult) *mockFetcher {
	return &mockFetcher{results: results}
}

func (m *mockFetcher) GetSecretValue(
	_ context.Context,
	_ *secretsmanager.GetSecretValueInput,
	_ ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls >= len(m.results) {
		// Repeat the last result if exhausted.
		r := m.results[len(m.results)-1]
		return r.output, r.err
	}
	r := m.results[m.calls]
	m.calls++
	return r.output, r.err
}

// generateCertPEMs returns (certPEM, keyPEM) as strings for test secret payloads.
func generateCertPEMs(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

// makeSecretJSON builds a Secrets Manager JSON payload from domain -> (certPEM, keyPEM).
func makeSecretJSON(t *testing.T, domains map[string][2]string) string {
	t.Helper()
	entries := make(map[string]secretEntry, len(domains))
	for domain, pair := range domains {
		entries[domain] = secretEntry{Cert: pair[0], Key: pair[1]}
	}
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal secret JSON: %v", err)
	}
	return string(b)
}

func nullLogger() zerolog.Logger { return zerolog.Nop() }

func selfSignedForTest(t *testing.T) tls.Certificate {
	t.Helper()
	cert, err := GenerateSelfSigned()
	if err != nil {
		t.Fatalf("generate self-signed: %v", err)
	}
	return cert
}

func okResult(s string) fetchResult {
	return fetchResult{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(s)}}
}

func errResult(err error) fetchResult {
	return fetchResult{err: err}
}

// TestProvider_NoSMConfig_SelfSigned verifies AC1 and S1: no SecretID means self-signed always.
func TestProvider_NoSMConfig_SelfSigned(t *testing.T) {
	t.Parallel()
	ss := selfSignedForTest(t)
	p := NewProvider(nil, "", "", time.Hour, ss, nullLogger())
	p.Start(context.Background())

	for _, sni := range []string{"", "smtp.example.com"} {
		cert, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: sni})
		if err != nil {
			t.Fatalf("GetCertificate(%q) error: %v", sni, err)
		}
		if cert != &p.selfSigned {
			t.Errorf("sni=%q: expected self-signed, got different cert", sni)
		}
	}
}

// TestProvider_SingleDomain_AC2 verifies AC2: single domain secret, cert served for SNI and no-SNI.
func TestProvider_SingleDomain_AC2(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"smtp.example.com": {certPEM, keyPEM},
	})
	mock := newMockFetcher(okResult(payload))
	ss := selfSignedForTest(t)
	p := NewProvider(mock, "my-secret", "", time.Hour, ss, nullLogger())
	p.Start(context.Background())

	// SNI matches → real cert.
	cert, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "smtp.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate(SNI): %v", err)
	}
	if cert == &p.selfSigned {
		t.Error("expected real cert for matching SNI, got self-signed")
	}

	// No SNI → default is that single domain's cert.
	cert2, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: ""})
	if err != nil {
		t.Fatalf("GetCertificate(no SNI): %v", err)
	}
	if cert2 == &p.selfSigned {
		t.Error("expected real cert for no-SNI single-domain, got self-signed")
	}
}

// TestProvider_MultiDomain_SNISelection_AC3 verifies AC3: SNI selects the correct domain cert.
func TestProvider_MultiDomain_SNISelection_AC3(t *testing.T) {
	t.Parallel()
	certA, keyA := generateCertPEMs(t)
	certB, keyB := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"a.example.com": {certA, keyA},
		"b.example.com": {certB, keyB},
	})
	mock := newMockFetcher(okResult(payload))
	ss := selfSignedForTest(t)
	p := NewProvider(mock, "my-secret", "", time.Hour, ss, nullLogger())
	p.Start(context.Background())

	certForA, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "a.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate(a): %v", err)
	}
	certForB, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "b.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate(b): %v", err)
	}

	if certForA == &p.selfSigned || certForB == &p.selfSigned {
		t.Error("expected real certs for both known domains, got self-signed for at least one")
	}
	if certForA == certForB {
		t.Error("expected distinct certs for a and b domains")
	}
}

// TestProvider_SNIMiss_SelfSigned_AC4 verifies AC4: unmatched SNI falls back to self-signed.
func TestProvider_SNIMiss_SelfSigned_AC4(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"a.example.com": {certPEM, keyPEM},
	})
	mock := newMockFetcher(okResult(payload))
	ss := selfSignedForTest(t)
	p := NewProvider(mock, "my-secret", "", time.Hour, ss, nullLogger())
	p.Start(context.Background())

	cert, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "unknown.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert != &p.selfSigned {
		t.Error("expected self-signed for unmatched SNI (AC4)")
	}
}

// TestProvider_NoSNIDefaultSelection_AC5 verifies AC5:
// explicit defaultDomain sets the no-SNI cert;
// unset defaultDomain uses the first key in sorted order.
func TestProvider_NoSNIDefaultSelection_AC5(t *testing.T) {
	t.Parallel()
	certA, keyA := generateCertPEMs(t)
	certB, keyB := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"a.example.com": {certA, keyA},
		"b.example.com": {certB, keyB},
	})

	t.Run("explicit_default_b", func(t *testing.T) {
		t.Parallel()
		mock := newMockFetcher(okResult(payload))
		ss := selfSignedForTest(t)
		p := NewProvider(mock, "my-secret", "b.example.com", time.Hour, ss, nullLogger())
		p.Start(context.Background())

		certExplicitB, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "b.example.com"})
		certNoSNI, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: ""})
		if err != nil {
			t.Fatalf("GetCertificate(no SNI): %v", err)
		}
		if certNoSNI != certExplicitB {
			t.Error("expected no-SNI default to equal explicitly configured b.example.com cert")
		}
	})

	t.Run("sorted_first_a", func(t *testing.T) {
		t.Parallel()
		mock := newMockFetcher(okResult(payload))
		ss := selfSignedForTest(t)
		p := NewProvider(mock, "my-secret", "", time.Hour, ss, nullLogger())
		p.Start(context.Background())

		// a.example.com < b.example.com alphabetically.
		certExplicitA, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "a.example.com"})
		certNoSNI, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: ""})
		if err != nil {
			t.Fatalf("GetCertificate(no SNI): %v", err)
		}
		if certNoSNI != certExplicitA {
			t.Error("expected no-SNI default to equal sorted-first (a.example.com) cert")
		}
	})
}

// TestProvider_StartupFailure_SelfSigned_AC6 verifies AC6: startup fetch error → self-signed, no crash.
func TestProvider_StartupFailure_SelfSigned_AC6(t *testing.T) {
	t.Parallel()
	mock := newMockFetcher(errResult(fmt.Errorf("no such secret")))
	ss := selfSignedForTest(t)
	p := NewProvider(mock, "missing-secret", "", time.Hour, ss, nullLogger())
	p.Start(context.Background())

	cert, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: ""})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert != &p.selfSigned {
		t.Error("expected self-signed after startup fetch failure (AC6)")
	}
}

// TestProvider_ReloadFailureRetainsLastGood_AC8 verifies AC8:
// reload failure after a successful load retains the last-good bundle.
func TestProvider_ReloadFailureRetainsLastGood_AC8(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"smtp.example.com": {certPEM, keyPEM},
	})
	// First call succeeds; second call fails.
	mock := newMockFetcher(
		okResult(payload),
		errResult(fmt.Errorf("transient error")),
	)
	ss := selfSignedForTest(t)
	interval := 20 * time.Millisecond
	p := NewProvider(mock, "my-secret", "", interval, ss, nullLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// After initial load: real cert.
	cert1, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "smtp.example.com"})
	if cert1 == &p.selfSigned {
		t.Fatal("expected real cert after successful initial load")
	}

	// Wait for the reload ticker to fire and fail.
	time.Sleep(4 * interval)

	// Bundle should still be the last-good real cert.
	cert2, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "smtp.example.com"})
	if cert2 == &p.selfSigned {
		t.Error("expected last-good cert retained after reload failure (AC8)")
	}
}

// TestProvider_SelfSignedPromotion_AC9 verifies AC9:
// after a startup failure, a later successful reload promotes to the real cert.
func TestProvider_SelfSignedPromotion_AC9(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"smtp.example.com": {certPEM, keyPEM},
	})
	// First call fails (startup falls back to self-signed); second call succeeds.
	mock := newMockFetcher(
		errResult(fmt.Errorf("secret not ready yet")),
		okResult(payload),
	)
	ss := selfSignedForTest(t)
	interval := 20 * time.Millisecond
	p := NewProvider(mock, "my-secret", "", interval, ss, nullLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// Immediately after start: self-signed because startup load failed.
	cert1, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "smtp.example.com"})
	if cert1 != &p.selfSigned {
		t.Fatal("expected self-signed immediately after failed startup load")
	}

	// Wait for the reload to succeed.
	time.Sleep(4 * interval)

	cert2, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "smtp.example.com"})
	if cert2 == &p.selfSigned {
		t.Error("expected promotion from self-signed to real cert after successful reload (AC9)")
	}
}

// TestProvider_PartialFailureIsolation_AC10 verifies AC10:
// invalid cert/key entry is skipped; valid entries are still served.
func TestProvider_PartialFailureIsolation_AC10(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := generateCertPEMs(t)

	// Build raw JSON with one valid and one invalid entry.
	raw := map[string]secretEntry{
		"bad.example.com":  {Cert: "not-a-cert", Key: "not-a-key"},
		"good.example.com": {Cert: certPEM, Key: keyPEM},
	}
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	mock := newMockFetcher(okResult(string(jsonBytes)))
	ss := selfSignedForTest(t)
	p := NewProvider(mock, "my-secret", "good.example.com", time.Hour, ss, nullLogger())
	p.Start(context.Background())

	// Valid domain: real cert.
	cert, err := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "good.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate(good): %v", err)
	}
	if cert == &p.selfSigned {
		t.Error("expected real cert for valid domain (AC10)")
	}

	// Bad domain was skipped → SNI miss → self-signed.
	cert2, _ := p.GetCertificate(&tls.ClientHelloInfo{ServerName: "bad.example.com"})
	if cert2 != &p.selfSigned {
		t.Error("expected self-signed for skipped bad domain (AC10)")
	}
}

// TestProvider_ReloadIntervalDefault_AC11 verifies AC11:
// zero and negative intervals default to 168 hours.
func TestProvider_ReloadIntervalDefault_AC11(t *testing.T) {
	t.Parallel()
	ss := selfSignedForTest(t)
	expected := time.Duration(defaultReloadHours) * time.Hour

	for _, iv := range []time.Duration{0, -1, -time.Hour} {
		p := NewProvider(nil, "", "", iv, ss, nullLogger())
		if p.reloadInterval != expected {
			t.Errorf("interval %v: expected default %v, got %v", iv, expected, p.reloadInterval)
		}
	}
}

// TestProvider_ConcurrencySafe_AC12 verifies AC12: no data races under concurrent GetCertificate + reload.
// Run with: go test -race ./internal/tlsutil/...
func TestProvider_ConcurrencySafe_AC12(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := generateCertPEMs(t)
	payload := makeSecretJSON(t, map[string][2]string{
		"smtp.example.com": {certPEM, keyPEM},
	})
	mock := newMockFetcher(okResult(payload))
	ss := selfSignedForTest(t)
	interval := 5 * time.Millisecond
	p := NewProvider(mock, "my-secret", "", interval, ss, nullLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			for range 50 {
				sni := "smtp.example.com"
				if i%2 == 0 {
					sni = ""
				}
				_, _ = p.GetCertificate(&tls.ClientHelloInfo{ServerName: sni})
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	wg.Wait()
}
