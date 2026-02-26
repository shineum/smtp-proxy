package msgstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// s3API defines the subset of the S3 client interface used by S3Store.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Store stores messages in an S3-compatible object store.
type S3Store struct {
	client s3API
	bucket string
	prefix string
}

// NewS3Store creates a new S3Store with the given client, bucket, and key prefix.
func NewS3Store(client s3API, bucket, prefix string) *S3Store {
	return &S3Store{client: client, bucket: bucket, prefix: prefix}
}

// NewS3StoreFromConfig creates a new S3Store from a Config, building a real AWS S3 client.
// It supports custom endpoints (e.g. MinIO) via Config.S3Endpoint.
func NewS3StoreFromConfig(cfg Config) (*S3Store, error) {
	ctx := context.Background()

	optFns := []func(*awsconfig.LoadOptions) error{}

	if cfg.S3Region != "" {
		optFns = append(optFns, awsconfig.WithRegion(cfg.S3Region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("msgstore: load aws config: %w", err)
	}

	s3OptFns := []func(*s3.Options){}

	if cfg.S3Endpoint != "" {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.BaseEndpoint = &cfg.S3Endpoint
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3OptFns...)
	return &S3Store{
		client: client,
		bucket: cfg.S3Bucket,
		prefix: cfg.S3Prefix,
	}, nil
}

// key returns the full S3 object key for the given message ID.
func (s *S3Store) key(messageID string) string {
	return s.prefix + messageID
}

// Put uploads message data to S3.
func (s *S3Store) Put(ctx context.Context, messageID string, data []byte) error {
	k := s.key(messageID)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &k,
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("msgstore: s3 put: %w", err)
	}
	return nil
}

// Get downloads message data from S3.
// Returns ErrNotFound if the object does not exist.
func (s *S3Store) Get(ctx context.Context, messageID string) ([]byte, error) {
	k := s.key(messageID)
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &k,
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("msgstore: s3 get: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("msgstore: s3 read body: %w", err)
	}
	return data, nil
}

// Delete removes a message from S3.
// S3 DeleteObject is already idempotent, so this always returns nil on success.
func (s *S3Store) Delete(ctx context.Context, messageID string) error {
	k := s.key(messageID)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &k,
	})
	if err != nil {
		return fmt.Errorf("msgstore: s3 delete: %w", err)
	}
	return nil
}
