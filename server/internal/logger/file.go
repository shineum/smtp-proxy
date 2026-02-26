package logger

import (
	"io"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileConfig holds configuration for file-based log output with rotation.
type FileConfig struct {
	// Path is the file path to write logs to.
	Path string
	// MaxSizeMB is the maximum size in megabytes before rotation.
	MaxSizeMB int
	// MaxFiles is the number of rotated files to retain.
	MaxFiles int
}

// NewFileWriter returns an io.Writer that writes to a rotating log file.
// It uses lumberjack for automatic rotation based on file size.
// Old rotated files are compressed with gzip.
func NewFileWriter(cfg FileConfig) io.Writer {
	return &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxFiles,
		Compress:   true,
	}
}
