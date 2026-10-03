// Package storage answers: how are file bytes persistently stored and retrieved across backends like local disk and S3?
package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// Backend abstracts a persistent storage provider.
type Backend interface {
	Name() string
	Put(ctx context.Context, id string, r io.Reader, size int64, mime string) error
	Get(ctx context.Context, id string) (io.ReadCloser, int64, error)
	GetRange(ctx context.Context, id string, offset, length int64) (io.ReadCloser, error)
	Delete(ctx context.Context, id string) error
	Exists(ctx context.Context, id string) (bool, error)
}

var (
	backendsMu sync.RWMutex
	backends   = make(map[string]Backend)
)

// RegisterBackend registers a storage backend by name.
func RegisterBackend(name string, b Backend) {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	backends[name] = b
}

// GetBackend retrieves a registered storage backend.
func GetBackend(name string) (Backend, error) {
	backendsMu.RLock()
	defer backendsMu.RUnlock()
	b, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("storage backend %q not registered", name)
	}
	return b, nil
}
