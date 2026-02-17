package routing

import "testing"

func TestRoutingRule_Validate_EmptyPrimaryProvider(t *testing.T) {
	rule := RoutingRule{
		TenantID:        "tenant-1",
		PrimaryProvider: "",
		FallbackOrder:   []string{"ses"},
	}

	err := rule.Validate()
	if err == nil {
		t.Fatal("expected error for empty primary provider, got nil")
	}

	want := "primary provider is required"
	if err.Error() != want {
		t.Errorf("expected error %q, got %q", want, err.Error())
	}
}

func TestRoutingRule_Validate_ValidPrimaryProvider(t *testing.T) {
	rule := RoutingRule{
		TenantID:        "tenant-1",
		PrimaryProvider: "sendgrid",
		FallbackOrder:   []string{"ses", "mailgun"},
	}

	err := rule.Validate()
	if err != nil {
		t.Errorf("expected no error for valid primary provider, got %v", err)
	}
}

func TestRoutingRule_Validate_NoFallbacks(t *testing.T) {
	rule := RoutingRule{
		TenantID:        "tenant-1",
		PrimaryProvider: "ses",
	}

	err := rule.Validate()
	if err != nil {
		t.Errorf("expected no error when fallbacks are empty, got %v", err)
	}
}

func TestRoutingRule_Validate_EmptyTenantID(t *testing.T) {
	rule := RoutingRule{
		TenantID:        "",
		PrimaryProvider: "sendgrid",
	}

	// Validate only checks PrimaryProvider; empty TenantID is allowed
	err := rule.Validate()
	if err != nil {
		t.Errorf("expected no error for empty tenant ID (only primary is checked), got %v", err)
	}
}
