package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistered(t *testing.T) {
	// Verify all metrics are registered with the default registry.
	// promauto registers metrics automatically, so this test verifies
	// the package initializes without panics or duplicate registration.

	tests := []struct {
		name   string
		metric prometheus.Collector
	}{
		{"SMTPConnectionsTotal", SMTPConnectionsTotal},
		{"SMTPActiveSessions", SMTPActiveSessions},
		{"SMTPAuthAttemptsTotal", SMTPAuthAttemptsTotal},
		{"SMTPMessageEnqueuedTotal", SMTPMessageEnqueuedTotal},
		{"SMTPMessageEnqueueDuration", SMTPMessageEnqueueDuration},
		{"APIRequestsTotal", APIRequestsTotal},
		{"APIRequestDuration", APIRequestDuration},
		{"APIAuthFailuresTotal", APIAuthFailuresTotal},
		{"DBConnectionsActive", DBConnectionsActive},
		{"DBConnectionsIdle", DBConnectionsIdle},
		{"DBQueryDuration", DBQueryDuration},
		{"DBErrorsTotal", DBErrorsTotal},
		{"QueueDepth", QueueDepth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.metric == nil {
				t.Errorf("%s is nil", tt.name)
			}
		})
	}
}

func TestSMTPConnectionsCounter(t *testing.T) {
	SMTPConnectionsTotal.WithLabelValues("accepted").Inc()
	SMTPConnectionsTotal.WithLabelValues("rejected").Inc()
	// No panic means labels are valid
}

func TestSMTPActiveSessions(t *testing.T) {
	SMTPActiveSessions.Set(42)
	SMTPActiveSessions.Inc()
	SMTPActiveSessions.Dec()
}

func TestAPIRequestsCounter(t *testing.T) {
	APIRequestsTotal.WithLabelValues("GET", "/api/v1/accounts", "200").Inc()
	APIRequestsTotal.WithLabelValues("POST", "/api/v1/accounts", "201").Inc()
}

func TestAPIRequestDuration(t *testing.T) {
	APIRequestDuration.WithLabelValues("GET", "/api/v1/accounts").Observe(0.05)
}

func TestDBMetrics(t *testing.T) {
	DBConnectionsActive.Set(10)
	DBConnectionsIdle.Set(5)
	DBQueryDuration.WithLabelValues("get_account_by_id").Observe(0.003)
	DBErrorsTotal.WithLabelValues("get_account_by_id").Inc()
}

func TestQueueDepth(t *testing.T) {
	QueueDepth.WithLabelValues("queued").Set(100)
	QueueDepth.WithLabelValues("processing").Set(5)
	QueueDepth.WithLabelValues("failed").Set(2)
}
