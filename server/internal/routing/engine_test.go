package routing

import (
	"context"
	"errors"
	"testing"
)

// mockHealthChecker is a test double for the HealthChecker interface.
type mockHealthChecker struct {
	healthy map[string]bool
}

func (m *mockHealthChecker) IsHealthy(name string) bool {
	return m.healthy[name]
}

func TestEngine_PrimaryHealthy(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": true,
			"ses":      true,
			"mailgun":  true,
		},
	}
	engine := NewEngine(hc)

	provider, err := engine.ResolveProvider(context.Background(), "any-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "sendgrid" {
		t.Errorf("expected primary provider sendgrid, got %s", provider)
	}
}

func TestEngine_PrimaryUnhealthy_FirstFallbackHealthy(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": false,
			"ses":      true,
			"mailgun":  true,
		},
	}
	engine := NewEngine(hc)

	provider, err := engine.ResolveProvider(context.Background(), "any-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default fallback order is ["ses", "mailgun", "msgraph"]
	if provider != "ses" {
		t.Errorf("expected first fallback ses, got %s", provider)
	}
}

func TestEngine_PrimaryAndFirstFallbackUnhealthy_SecondFallbackHealthy(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": false,
			"ses":      false,
			"mailgun":  true,
			"msgraph":  false,
		},
	}
	engine := NewEngine(hc)

	provider, err := engine.ResolveProvider(context.Background(), "any-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "mailgun" {
		t.Errorf("expected second fallback mailgun, got %s", provider)
	}
}

func TestEngine_AllProvidersUnhealthy(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": false,
			"ses":      false,
			"mailgun":  false,
			"msgraph":  false,
		},
	}
	engine := NewEngine(hc)

	_, err := engine.ResolveProvider(context.Background(), "any-tenant")
	if err == nil {
		t.Fatal("expected ErrNoHealthyProvider, got nil")
	}
	if !errors.Is(err, ErrNoHealthyProvider) {
		t.Errorf("expected ErrNoHealthyProvider, got %v", err)
	}
}

func TestEngine_TenantSpecificRule(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": true,
			"ses":      true,
			"mailgun":  true,
			"custom":   true,
		},
	}
	engine := NewEngine(hc)

	// Set a tenant-specific rule that overrides the default
	engine.SetRule(&RoutingRule{
		TenantID:        "tenant-A",
		PrimaryProvider: "custom",
		FallbackOrder:   []string{"mailgun"},
	})

	provider, err := engine.ResolveProvider(context.Background(), "tenant-A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "custom" {
		t.Errorf("expected tenant-specific primary custom, got %s", provider)
	}
}

func TestEngine_DefaultRuleUsedWhenNoTenantRule(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": true,
			"ses":      true,
		},
	}
	engine := NewEngine(hc)

	// Set a rule for a different tenant only
	engine.SetRule(&RoutingRule{
		TenantID:        "tenant-A",
		PrimaryProvider: "ses",
		FallbackOrder:   []string{"sendgrid"},
	})

	// Query for a tenant without a specific rule -- should use default
	provider, err := engine.ResolveProvider(context.Background(), "tenant-B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "sendgrid" {
		t.Errorf("expected default primary sendgrid for unknown tenant, got %s", provider)
	}
}

func TestEngine_SetRule_ModifiesBehavior(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": true,
			"ses":      true,
			"mailgun":  true,
		},
	}
	engine := NewEngine(hc)

	// Initially, default rule applies
	provider, err := engine.ResolveProvider(context.Background(), "tenant-X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "sendgrid" {
		t.Fatalf("expected sendgrid before SetRule, got %s", provider)
	}

	// Set a tenant-specific rule
	engine.SetRule(&RoutingRule{
		TenantID:        "tenant-X",
		PrimaryProvider: "mailgun",
		FallbackOrder:   []string{"ses"},
	})

	provider, err = engine.ResolveProvider(context.Background(), "tenant-X")
	if err != nil {
		t.Fatalf("unexpected error after SetRule: %v", err)
	}
	if provider != "mailgun" {
		t.Errorf("expected mailgun after SetRule, got %s", provider)
	}
}

func TestEngine_SetDefaultRule_ModifiesBehavior(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"ses":     true,
			"mailgun": true,
		},
	}
	engine := NewEngine(hc)

	// Override the default rule
	engine.SetDefaultRule(&RoutingRule{
		PrimaryProvider: "ses",
		FallbackOrder:   []string{"mailgun"},
	})

	provider, err := engine.ResolveProvider(context.Background(), "unknown-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "ses" {
		t.Errorf("expected ses as new default, got %s", provider)
	}
}

func TestEngine_TenantFallbackChain(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"primary":   false,
			"fallback1": false,
			"fallback2": true,
		},
	}
	engine := NewEngine(hc)

	engine.SetRule(&RoutingRule{
		TenantID:        "tenant-chain",
		PrimaryProvider: "primary",
		FallbackOrder:   []string{"fallback1", "fallback2"},
	})

	provider, err := engine.ResolveProvider(context.Background(), "tenant-chain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "fallback2" {
		t.Errorf("expected fallback2, got %s", provider)
	}
}

func TestEngine_EmptyFallbackOrder_AllUnhealthy(t *testing.T) {
	hc := &mockHealthChecker{
		healthy: map[string]bool{
			"sendgrid": false,
		},
	}
	engine := NewEngine(hc)

	engine.SetRule(&RoutingRule{
		TenantID:        "tenant-no-fallback",
		PrimaryProvider: "sendgrid",
		FallbackOrder:   nil,
	})

	_, err := engine.ResolveProvider(context.Background(), "tenant-no-fallback")
	if !errors.Is(err, ErrNoHealthyProvider) {
		t.Errorf("expected ErrNoHealthyProvider when no fallbacks, got %v", err)
	}
}
