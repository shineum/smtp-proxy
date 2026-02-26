package msgstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LocalFileStore stores messages as files on the local filesystem.
type LocalFileStore struct {
	basePath string
}

// NewLocalFileStore creates a new LocalFileStore at the given base path.
// It creates the directory if it does not exist.
func NewLocalFileStore(basePath string) (*LocalFileStore, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("msgstore: create base directory: %w", err)
	}
	return &LocalFileStore{basePath: basePath}, nil
}

// Put writes message data to a file using an atomic write pattern.
func (s *LocalFileStore) Put(_ context.Context, messageID string, data []byte) error {
	finalPath := filepath.Join(s.basePath, messageID)

	// Write to a temp file in the same directory, then rename for atomicity.
	tmp, err := os.CreateTemp(s.basePath, ".tmp-"+messageID+"-*")
	if err != nil {
		return fmt.Errorf("msgstore: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("msgstore: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("msgstore: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("msgstore: rename temp file: %w", err)
	}
	return nil
}

// Get reads message data from a file.
// Returns ErrNotFound if the message does not exist.
func (s *LocalFileStore) Get(_ context.Context, messageID string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.basePath, messageID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("msgstore: read file: %w", err)
	}
	return data, nil
}

// Delete removes a message file.
// Returns nil if the message does not exist (idempotent).
func (s *LocalFileStore) Delete(_ context.Context, messageID string) error {
	err := os.Remove(filepath.Join(s.basePath, messageID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("msgstore: remove file: %w", err)
	}
	return nil
}
