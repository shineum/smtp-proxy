package logger

import (
	"fmt"
	"io"
	"os"
)

// CloudWatchConfig holds configuration for CloudWatch Logs output.
type CloudWatchConfig struct {
	// Group is the CloudWatch log group name.
	Group string
	// Stream is the CloudWatch log stream name.
	Stream string
	// Region is the AWS region for the CloudWatch endpoint.
	Region string
}

// cloudWatchWriter is a stub implementation of io.Writer for CloudWatch Logs.
// It currently writes to stdout as a fallback. A full implementation using
// aws-sdk-go-v2/service/cloudwatchlogs should replace this when the
// dependency is confirmed and integrated.
type cloudWatchWriter struct {
	cfg    CloudWatchConfig
	stdout io.Writer
}

// NewCloudWatchWriter returns an io.Writer that is intended to send log
// entries to AWS CloudWatch Logs. This is currently a stub that writes
// to stdout. The full CloudWatch SDK integration can be added later
// without changing the caller interface.
func NewCloudWatchWriter(cfg CloudWatchConfig) io.Writer {
	// TODO: Replace stdout fallback with actual CloudWatch Logs PutLogEvents
	// calls using aws-sdk-go-v2/service/cloudwatchlogs. The writer should
	// batch log entries and flush them periodically or when a size threshold
	// is reached.
	fmt.Fprintf(os.Stderr,
		"cloudwatch output configured (group=%s, stream=%s, region=%s) but not yet implemented; falling back to stdout\n",
		cfg.Group, cfg.Stream, cfg.Region,
	)
	return &cloudWatchWriter{
		cfg:    cfg,
		stdout: os.Stdout,
	}
}

// Write implements io.Writer. Currently delegates to stdout.
func (w *cloudWatchWriter) Write(p []byte) (n int, err error) {
	return w.stdout.Write(p)
}
