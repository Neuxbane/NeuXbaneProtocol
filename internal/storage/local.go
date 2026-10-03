package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func init() {
	defaultDir := filepath.Join(os.TempDir(), "nxp-storage")
	RegisterBackend("local", NewLocalBackend(defaultDir))
}

// LocalBackend implements Backend on the local filesystem.
type LocalBackend struct {
	baseDir string
}

// NewLocalBackend initializes a LocalBackend at baseDir.
func NewLocalBackend(baseDir string) *LocalBackend {
	_ = os.MkdirAll(baseDir, 0755)
	return &LocalBackend{baseDir: baseDir}
}

// Name implements Backend.
func (l *LocalBackend) Name() string {
	return "local"
}

func (l *LocalBackend) filePath(id string) string {
	// Partition by first two chars of id to avoid large flat directories
	part := "default"
	if len(id) >= 2 {
		part = id[:2]
	}
	return filepath.Join(l.baseDir, part, id)
}

// Put saves data atomically to disk.
func (l *LocalBackend) Put(ctx context.Context, id string, r io.Reader, size int64, mime string) error {
	dest := l.filePath(id)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("local put mkdir: %w", err)
	}

	tmpFile := dest + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("local put open tmp: %w", err)
	}

	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("local put write: %w", err)
	}
	_ = f.Close()

	return os.Rename(tmpFile, dest)
}

// Get returns an io.ReadCloser for the stored file.
func (l *LocalBackend) Get(ctx context.Context, id string) (io.ReadCloser, int64, error) {
	p := l.filePath(id)
	info, err := os.Stat(p)
	if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, 0, err
	}

	return f, info.Size(), nil
}

// GetRange returns a section of the stored file.
func (l *LocalBackend) GetRange(ctx context.Context, id string, offset, length int64) (io.ReadCloser, error) {
	p := l.filePath(id)
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}

	return &limitReadCloser{
		Reader: io.LimitReader(f, length),
		Closer: f,
	}, nil
}

// Delete removes the file from disk.
func (l *LocalBackend) Delete(ctx context.Context, id string) error {
	return os.Remove(l.filePath(id))
}

// Exists checks if the file exists.
func (l *LocalBackend) Exists(ctx context.Context, id string) (bool, error) {
	_, err := os.Stat(l.filePath(id))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

type limitReadCloser struct {
	io.Reader
	io.Closer
}
