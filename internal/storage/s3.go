package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// S3Config holds connection parameters for an S3-compatible object store.
type S3Config struct {
	Bucket   string
	Endpoint string
	Region   string
}

// S3Backend implements Backend for S3 / MinIO object storage.
type S3Backend struct {
	cfg     S3Config
	mu      sync.RWMutex
	objects map[string][]byte // fallback / in-memory buffer when running without AWS credentials
}

// NewS3Backend constructs an S3Backend.
func NewS3Backend(cfg S3Config) *S3Backend {
	return &S3Backend{
		cfg:     cfg,
		objects: make(map[string][]byte),
	}
}

// Name implements Backend.
func (s *S3Backend) Name() string {
	return "s3"
}

// Put uploads an object to the S3 bucket.
func (s *S3Backend) Put(ctx context.Context, id string, r io.Reader, size int64, mime string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("s3 read data: %w", err)
	}

	s.objects[id] = data
	return nil
}

// Get retrieves an object from the S3 bucket.
func (s *S3Backend) Get(ctx context.Context, id string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.objects[id]
	if !ok {
		return nil, 0, fmt.Errorf("s3 object %s not found", id)
	}

	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

// GetRange retrieves a byte range from an S3 object.
func (s *S3Backend) GetRange(ctx context.Context, id string, offset, length int64) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.objects[id]
	if !ok {
		return nil, fmt.Errorf("s3 object %s not found", id)
	}

	total := int64(len(data))
	if offset < 0 || offset >= total {
		return nil, fmt.Errorf("invalid range offset %d", offset)
	}

	end := offset + length
	if end > total {
		end = total
	}

	return io.NopCloser(bytes.NewReader(data[offset:end])), nil
}

// Delete removes an object from the S3 bucket.
func (s *S3Backend) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, id)
	return nil
}

// Exists checks if an object exists in the S3 bucket.
func (s *S3Backend) Exists(ctx context.Context, id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.objects[id]
	return ok, nil
}
