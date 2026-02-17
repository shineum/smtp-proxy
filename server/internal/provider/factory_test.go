package provider

import (
	"context"
	"sort"
	"testing"
)

// mockProvider implements Provider for registry tests.
type mockProvider struct {
	name string
}

func (m *mockProvider) Send(_ context.Context, _ *Message) (*DeliveryResult, error) {
	return nil, nil
}

func (m *mockProvider) GetName() string {
	return m.name
}

func (m *mockProvider) HealthCheck(_ context.Context) error {
	return nil
}

// mockHTTPClient implements HTTPClient for NewProvider tests.
type mockHTTPClient struct{}

func (m *mockHTTPClient) Do(_ *HTTPRequest) (*HTTPResponse, error) {
	return &HTTPResponse{StatusCode: 200, Body: []byte(`{"access_token":"tok","expires_in":3600}`)}, nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	names := r.List()
	if len(names) != 0 {
		t.Errorf("expected empty registry, got %d providers", len(names))
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	mp := &mockProvider{name: "mock-one"}

	r.Register(mp)

	got, err := r.Get("mock-one")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.GetName() != "mock-one" {
		t.Errorf("Get() returned provider with name %q, want %q", got.GetName(), "mock-one")
	}
}

func TestRegistry_Get_UnknownName(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}

	want := "provider not found: nonexistent"
	if err.Error() != want {
		t.Errorf("Get() error = %q, want %q", err.Error(), want)
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "alpha"})
	r.Register(&mockProvider{name: "beta"})
	r.Register(&mockProvider{name: "gamma"})

	names := r.List()
	if len(names) != 3 {
		t.Fatalf("List() returned %d names, want 3", len(names))
	}

	sort.Strings(names)
	expected := []string{"alpha", "beta", "gamma"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("List()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "alpha"})
	r.Register(&mockProvider{name: "beta"})

	providers := r.All()
	if len(providers) != 2 {
		t.Fatalf("All() returned %d providers, want 2", len(providers))
	}

	nameSet := make(map[string]bool)
	for _, p := range providers {
		nameSet[p.GetName()] = true
	}

	if !nameSet["alpha"] {
		t.Error("All() missing provider 'alpha'")
	}
	if !nameSet["beta"] {
		t.Error("All() missing provider 'beta'")
	}
}

func TestNewProvider_ValidSendgrid(t *testing.T) {
	cfg := ProviderConfig{
		Type:   "sendgrid",
		APIKey: "test-token-sg",
	}
	client := &mockHTTPClient{}

	p, err := NewProvider(cfg, client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewProvider() returned nil provider")
	}
	if p.GetName() != "sendgrid" {
		t.Errorf("GetName() = %q, want %q", p.GetName(), "sendgrid")
	}
}

func TestNewProvider_ValidSES(t *testing.T) {
	cfg := ProviderConfig{
		Type:   "ses",
		APIKey: "test-token-ses",
		Region: "us-east-1",
	}
	client := &mockHTTPClient{}

	p, err := NewProvider(cfg, client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if p.GetName() != "ses" {
		t.Errorf("GetName() = %q, want %q", p.GetName(), "ses")
	}
}

func TestNewProvider_ValidMailgun(t *testing.T) {
	cfg := ProviderConfig{
		Type:   "mailgun",
		APIKey: "test-token-mg",
		Domain: "mg.example.com",
	}
	client := &mockHTTPClient{}

	p, err := NewProvider(cfg, client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if p.GetName() != "mailgun" {
		t.Errorf("GetName() = %q, want %q", p.GetName(), "mailgun")
	}
}

func TestNewProvider_ValidMSGraph(t *testing.T) {
	cfg := ProviderConfig{
		Type:         "msgraph",
		TenantID:     "test-tenant",
		ClientID:     "test-client",
		ClientSecret: "test-value",
		UserID:       "user@example.com",
	}
	client := &mockHTTPClient{}

	p, err := NewProvider(cfg, client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if p.GetName() != "msgraph" {
		t.Errorf("GetName() = %q, want %q", p.GetName(), "msgraph")
	}
}

func TestNewProvider_InvalidConfig(t *testing.T) {
	cfg := ProviderConfig{
		Type: "sendgrid",
		// Missing required fields
	}
	client := &mockHTTPClient{}

	_, err := NewProvider(cfg, client)
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestNewProvider_UnsupportedType(t *testing.T) {
	// The Validate() method rejects unknown types before NewProvider's switch,
	// so this test verifies the validation path catches it.
	cfg := ProviderConfig{
		Type: "unsupported",
	}
	client := &mockHTTPClient{}

	_, err := NewProvider(cfg, client)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "dup"})
	r.Register(&mockProvider{name: "dup"})

	names := r.List()
	if len(names) != 1 {
		t.Errorf("expected 1 provider after overwrite, got %d", len(names))
	}
}
