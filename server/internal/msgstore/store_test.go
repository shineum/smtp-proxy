package msgstore

import (
	"errors"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestErrNotFound_Is(t *testing.T) {
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("errors.Is(ErrNotFound, ErrNotFound) should be true")
	}
}

func TestNew_LocalDefault(t *testing.T) {
	dir := t.TempDir()
	logger := zerolog.New(os.Stderr)

	store, err := New(Config{Type: "", Path: dir}, logger)
	if err != nil {
		t.Fatalf("New with empty type: %v", err)
	}
	if store == nil {
		t.Fatal("New with empty type returned nil store")
	}
	if _, ok := store.(*LocalFileStore); !ok {
		t.Errorf("New with empty type: got %T, want *LocalFileStore", store)
	}
}

func TestNew_LocalExplicit(t *testing.T) {
	dir := t.TempDir()
	logger := zerolog.New(os.Stderr)

	store, err := New(Config{Type: "local", Path: dir}, logger)
	if err != nil {
		t.Fatalf("New with type=local: %v", err)
	}
	if store == nil {
		t.Fatal("New with type=local returned nil store")
	}
	if _, ok := store.(*LocalFileStore); !ok {
		t.Errorf("New with type=local: got %T, want *LocalFileStore", store)
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	dir := t.TempDir()
	logger := zerolog.New(os.Stderr)

	store, err := New(Config{Type: "gcs", Path: dir}, logger)
	if err != nil {
		t.Fatalf("New with type=gcs: %v", err)
	}
	if store == nil {
		t.Fatal("New with type=gcs returned nil store")
	}
	if _, ok := store.(*LocalFileStore); !ok {
		t.Errorf("New with type=gcs: got %T, want *LocalFileStore", store)
	}
}
