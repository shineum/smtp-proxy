package routing

import (
	"context"
	"errors"
	"sync"
)

// ErrNoHealthyProvider is returned when no healthy provider is available.
var ErrNoHealthyProvider = errors.New("no healthy provider available")

// HealthChecker reports whether a named provider is currently healthy.
type HealthChecker interface {
	IsHealthy(providerName string) bool
}

// Engine resolves the ESP provider for a given tenant using routing rules
// and provider health status.
type Engine struct {
	rules         map[string]*RoutingRule
	healthChecker HealthChecker
	mu            sync.RWMutex
	defaultRule   *RoutingRule
}

// NewEngine creates a routing engine that uses the given health checker
// to determine provider availability.
func NewEngine(healthChecker HealthChecker) *Engine {
	return &Engine{
		rules:         make(map[string]*RoutingRule),
		healthChecker: healthChecker,
		defaultRule: &RoutingRule{
			PrimaryProvider: "sendgrid",
			FallbackOrder:   []string{"ses", "mailgun", "msgraph"},
		},
	}
}

// SetRule adds or updates the routing rule for a tenant.
func (e *Engine) SetRule(rule *RoutingRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[rule.TenantID] = rule
}

// SetDefaultRule replaces the default routing rule used when no
// tenant-specific rule exists.
func (e *Engine) SetDefaultRule(rule *RoutingRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.defaultRule = rule
}

// ResolveProvider selects a healthy ESP provider for the given tenant.
// It checks the primary provider first, then iterates through the fallback
// order. Returns ErrNoHealthyProvider if every candidate is unhealthy.
func (e *Engine) ResolveProvider(_ context.Context, tenantID string) (string, error) {
	e.mu.RLock()
	rule, ok := e.rules[tenantID]
	if !ok {
		rule = e.defaultRule
	}
	e.mu.RUnlock()

	if e.healthChecker.IsHealthy(rule.PrimaryProvider) {
		return rule.PrimaryProvider, nil
	}

	for _, name := range rule.FallbackOrder {
		if e.healthChecker.IsHealthy(name) {
			return name, nil
		}
	}

	return "", ErrNoHealthyProvider
}
