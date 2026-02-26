package msgstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3Client implements the s3API interface for testing.
type mockS3Client struct {
	objects map[string][]byte
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{objects: make(map[string][]byte)}
}

func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	key := ""
	if params.Key != nil {
		key = *params.Key
	}
	data, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	m.objects[key] = data
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := ""
	if params.Key != nil {
		key = *params.Key
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, &types.NoSuchKey{Message: stringPtr(fmt.Sprintf("key %q not found", key))}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	key := ""
	if params.Key != nil {
		key = *params.Key
	}
	delete(m.objects, key)
	return &s3.DeleteObjectOutput{}, nil
}

func stringPtr(s string) *string { return &s }

func TestS3Store_PutAndGet(t *testing.T) {
	mock := newMockS3Client()
	store := NewS3Store(mock, "test-bucket", "prefix/")

	ctx := context.Background()
	data := []byte("s3 test data")

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

func TestS3Store_GetNotFound(t *testing.T) {
	mock := newMockS3Client()
	store := NewS3Store(mock, "test-bucket", "prefix/")

	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get non-existent: got err=%v, want ErrNotFound", err)
	}
}

func TestS3Store_DeleteExisting(t *testing.T) {
	mock := newMockS3Client()
	store := NewS3Store(mock, "test-bucket", "prefix/")

	ctx := context.Background()
	data := []byte("to be deleted")

	if err := store.Put(ctx, "msg-del", data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := store.Delete(ctx, "msg-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(ctx, "msg-del")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: got err=%v, want ErrNotFound", err)
	}
}

func TestS3Store_DeleteIdempotent(t *testing.T) {
	mock := newMockS3Client()
	store := NewS3Store(mock, "test-bucket", "prefix/")

	ctx := context.Background()
	// S3 DeleteObject is already idempotent; deleting a nonexistent key should return nil.
	if err := store.Delete(ctx, "never-existed"); err != nil {
		t.Errorf("Delete non-existent: got err=%v, want nil", err)
	}
}

func TestS3Store_KeyPrefix(t *testing.T) {
	mock := newMockS3Client()
	store := NewS3Store(mock, "test-bucket", "emails/")

	ctx := context.Background()
	data := []byte("prefix test")

	if err := store.Put(ctx, "msg-pfx", data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Verify the key in the mock includes the prefix.
	expectedKey := "emails/msg-pfx"
	if _, ok := mock.objects[expectedKey]; !ok {
		keys := make([]string, 0, len(mock.objects))
		for k := range mock.objects {
			keys = append(keys, k)
		}
		t.Errorf("expected key %q in mock objects, got keys: %v", expectedKey, keys)
	}
}
