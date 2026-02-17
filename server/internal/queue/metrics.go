package queue

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Queue metrics for Prometheus monitoring.
var (
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "queue_messages_pending",
			Help: "Number of pending messages in queue per tenant",
		},
		[]string{"tenant_id"},
	)

	MessagesEnqueuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "queue_messages_enqueued_total",
			Help: "Total number of messages enqueued",
		},
	)

	MessagesProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_messages_processed_total",
			Help: "Total number of messages processed by status",
		},
		[]string{"status"}, // sent, failed, dlq
	)

	MessageProcessingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "queue_message_processing_duration_seconds",
			Help:    "Duration of message processing operations",
			Buckets: prometheus.DefBuckets,
		},
	)

	DLQMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_dlq_messages_total",
			Help: "Total number of messages moved to DLQ by reason",
		},
		[]string{"reason"},
	)
)
