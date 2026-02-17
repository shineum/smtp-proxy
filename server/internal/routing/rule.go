package routing

import "errors"

// RoutingRule defines the ESP provider selection strategy for a tenant.
type RoutingRule struct {
	TenantID        string
	PrimaryProvider string
	FallbackOrder   []string
}

// Validate checks that the routing rule has a primary provider configured.
func (r *RoutingRule) Validate() error {
	if r.PrimaryProvider == "" {
		return errors.New("primary provider is required")
	}
	return nil
}
