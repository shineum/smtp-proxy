package provider

import (
	"context"
	"sync"
	"time"
)

const (
	defaultCheckInterval    = 30 * time.Second
	defaultCheckTimeout     = 10 * time.Second
	unhealthyThreshold      = 3
)

// HealthStatus represents the current health state of a provider.
type HealthStatus struct {
	Healthy             bool
	LastCheck           time.Time
	ConsecutiveFailures int
	LastError           string
}

// HealthChecker periodically checks provider health and tracks status.
type HealthChecker struct {
	mu            sync.RWMutex
	registry      *Registry
	statuses      map[string]*HealthStatus
	checkInterval time.Duration
	checkTimeout  time.Duration
	stopCh        chan struct{}
	stopped       chan struct{}
}

// NewHealthChecker creates a health checker that monitors all providers
// in the given registry.
func NewHealthChecker(registry *Registry) *HealthChecker {
	return &HealthChecker{
		registry:      registry,
		statuses:      make(map[string]*HealthStatus),
		checkInterval: defaultCheckInterval,
		checkTimeout:  defaultCheckTimeout,
		stopCh:        make(chan struct{}),
		stopped:       make(chan struct{}),
	}
}

// Start begins the background health check loop.
func (hc *HealthChecker) Start() {
	go hc.run()
}

// Stop signals the health check loop to terminate and waits for it to finish.
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
	<-hc.stopped
}

// IsHealthy returns whether a provider is currently healthy.
func (hc *HealthChecker) IsHealthy(name string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	status, ok := hc.statuses[name]
	if !ok {
		// Unknown provider is considered unhealthy.
		return false
	}
	return status.Healthy
}

// GetStatus returns the full health status for a provider.
func (hc *HealthChecker) GetStatus(name string) (HealthStatus, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	status, ok := hc.statuses[name]
	if !ok {
		return HealthStatus{}, false
	}
	return *status, true
}

// GetAllStatuses returns a snapshot of all provider health statuses.
func (hc *HealthChecker) GetAllStatuses() map[string]HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result := make(map[string]HealthStatus, len(hc.statuses))
	for name, status := range hc.statuses {
		result[name] = *status
	}
	return result
}

func (hc *HealthChecker) run() {
	defer close(hc.stopped)

	// Run an initial check immediately.
	hc.checkAll()

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

func (hc *HealthChecker) checkAll() {
	providers := hc.registry.All()
	for _, p := range providers {
		hc.checkProvider(p)
	}
}

func (hc *HealthChecker) checkProvider(p Provider) {
	ctx, cancel := context.WithTimeout(context.Background(), hc.checkTimeout)
	defer cancel()

	err := p.HealthCheck(ctx)
	name := p.GetName()

	hc.mu.Lock()
	defer hc.mu.Unlock()

	status, ok := hc.statuses[name]
	if !ok {
		status = &HealthStatus{Healthy: true}
		hc.statuses[name] = status
	}

	status.LastCheck = time.Now()

	if err != nil {
		status.ConsecutiveFailures++
		status.LastError = err.Error()
		if status.ConsecutiveFailures >= unhealthyThreshold {
			status.Healthy = false
		}
	} else {
		// 1 success resets to healthy.
		status.ConsecutiveFailures = 0
		status.Healthy = true
		status.LastError = ""
	}
}
