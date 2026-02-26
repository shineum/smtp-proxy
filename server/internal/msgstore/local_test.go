package msgstore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestLocalFileStore_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalFileStore(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStore: %v", err)
	}

	ctx := context.Background()
	data := []byte("hello, world")

	if err := store.Put(ctx, "msg-001", data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get(ctx, "msg-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("Get = %q, want %q", got, data)
	}
}

func TestLocalFileStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalFileStore(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStore: %v", err)
	}

	ctx := context.Background()
	_, err = store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get non-existent: got err=%v, want ErrNotFound", err)
	}
}

func TestLocalFileStore_DeleteExisting(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalFileStore(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStore: %v", err)
	}

	ctx := context.Background()
	data := []byte("to be deleted")

	if err := store.Put(ctx, "msg-del", data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := store.Delete(ctx, "msg-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get(ctx, "msg-del")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: got err=%v, want ErrNotFound", err)
	}
}

func TestLocalFileStore_DeleteIdempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalFileStore(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStore: %v", err)
	}

	ctx := context.Background()
	if err := store.Delete(ctx, "never-existed"); err != nil {
		t.Errorf("Delete non-existent: got err=%v, want nil", err)
	}
}

func TestLocalFileStore_AutoCreateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	store, err := NewLocalFileStore(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStore with nested dir: %v", err)
	}

	ctx := context.Background()
	data := []byte("nested dir test")

	if err := store.Put(ctx, "msg-nested", data); err != nil {
		t.Fatalf("Put after auto-create: %v", err)
	}

	got, err := store.Get(ctx, "msg-nested")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Get = %q, want %q", got, data)
	}
}

func TestLocalFileStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalFileStore(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStore: %v", err)
	}

	ctx := context.Background()
	const n = 50
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msgID := "msg-" + string(rune('A'+id%26)) + "-" + itoa(id)
			data := []byte("data-" + itoa(id))
			if err := store.Put(ctx, msgID, data); err != nil {
				t.Errorf("concurrent Put(%s): %v", msgID, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msgID := "msg-" + string(rune('A'+id%26)) + "-" + itoa(id)
			expected := []byte("data-" + itoa(id))
			got, err := store.Get(ctx, msgID)
			if err != nil {
				t.Errorf("concurrent Get(%s): %v", msgID, err)
				return
			}
			if string(got) != string(expected) {
				t.Errorf("concurrent Get(%s) = %q, want %q", msgID, got, expected)
			}
		}(i)
	}
	wg.Wait()
}

// itoa is a simple int-to-string helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
