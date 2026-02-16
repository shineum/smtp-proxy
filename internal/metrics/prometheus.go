package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SMTP metrics
var (
	SMTPConnectionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smtp_connections_total",
			Help: "Total number of SMTP connections",
		},
		[]string{"status"}, // accepted, rejected
	)

	SMTPActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "smtp_active_sessions",
			Help: "Number of currently active SMTP sessions",
		},
	)

	SMTPAuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smtp_auth_attempts_total",
			Help: "Total number of SMTP authentication attempts",
		},
		[]string{"result"}, // success, failure
	)

	SMTPMessageEnqueuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "smtp_message_enqueued_total",
			Help: "Total number of messages enqueued via SMTP",
		},
	)

	SMTPMessageEnqueueDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "smtp_message_enqueue_duration_seconds",
			Help:    "Duration of message enqueue operations",
			Buckets: prometheus.DefBuckets,
		},
	)
)

// API metrics
var (
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "path", "status"},
	)

	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Duration of API requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	APIAuthFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "api_auth_failures_total",
			Help: "Total number of API authentication failures",
		},
	)
)

// Database metrics
var (
	DBConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
	)

	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duration of database queries",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"query"},
	)

	DBErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_errors_total",
			Help: "Total number of database errors",
		},
		[]string{"query"},
	)
)

// Queue metrics
var (
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "queue_depth",
			Help: "Number of messages in queue by status",
		},
		[]string{"status"}, // queued, processing, failed
	)
)
