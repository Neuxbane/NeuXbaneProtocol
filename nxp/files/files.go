// Package files provides developer-facing file upload, download, and storage primitives.
package files

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/storage"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/upload"
)

// UploadResult is returned by upload handlers to describe stored file metadata.
type UploadResult struct {
	ID   string `json:"id"   validate:"required"`
	Size int64  `json:"size" validate:"min=0"`
	Name string `json:"name" validate:"required"`
}

// DownloadResult defines the source file or presigned URL to stream back to the client.
type DownloadResult struct {
	Source    string
	Name      string
	Presigned bool
}

// UploadCtx represents the file upload execution context available to upload handlers.
type UploadCtx struct {
	file     *upload.ReceivedFile
	fields   map[string]string
	reader   *os.File
	backend  storage.Backend
	discarded bool
}

// NewUploadCtx constructs an UploadCtx.
func NewUploadCtx(file *upload.ReceivedFile, fields map[string]string, backend storage.Backend) (*UploadCtx, error) {
	var f *os.File
	if file != nil && file.TempPath != "" {
		var err error
		f, err = os.Open(file.TempPath)
		if err != nil {
			return nil, err
		}
	}

	return &UploadCtx{
		file:    file,
		fields:  fields,
		reader:  f,
		backend: backend,
	}, nil
}

// SHA256 returns the calculated SHA256 checksum of the uploaded file.
func (c *UploadCtx) SHA256() string {
	if c.file == nil {
		return ""
	}
	return c.file.SHA256
}

// Size returns the file size in bytes.
func (c *UploadCtx) Size() int64 {
	if c.file == nil {
		return 0
	}
	return c.file.Size
}

// Mime returns the sniffed MIME type of the uploaded payload.
func (c *UploadCtx) Mime() string {
	if c.file == nil {
		return "application/octet-stream"
	}
	return c.file.Mime
}

// SafeName returns the sanitized, path-traversal-free filename.
func (c *UploadCtx) SafeName() string {
	if c.file == nil {
		return "unnamed_file"
	}
	return c.file.SafeName
}

// OriginalName returns the original client-provided filename.
func (c *UploadCtx) OriginalName() string {
	if c.file == nil {
		return ""
	}
	return c.file.OriginalName
}

// Quarantine reports whether the file was quarantined due to security policies.
func (c *UploadCtx) Quarantine() bool {
	if c.file == nil {
		return false
	}
	return upload.ShouldQuarantine(c.file.OriginalName, c.file.Mime)
}

// Field retrieves a multipart form field value by name.
func (c *UploadCtx) Field(name string) string {
	if c.fields == nil {
		return ""
	}
	return c.fields[name]
}

// Reader provides an io.Reader stream to read the uploaded bytes.
func (c *UploadCtx) Reader() io.Reader {
	return c.reader
}

// Seek repositions the file read offset.
func (c *UploadCtx) Seek(offset int64) error {
	if c.reader == nil {
		return fmt.Errorf("no open file reader")
	}
	_, err := c.reader.Seek(offset, io.SeekStart)
	return err
}

// MoveTo relocates the temporary upload to a permanent storage destination ID.
func (c *UploadCtx) MoveTo(dest string) error {
	if c.backend == nil {
		b, err := storage.GetBackend("local")
		if err != nil {
			return err
		}
		c.backend = b
	}

	if err := c.Seek(0); err != nil {
		return err
	}

	err := c.backend.Put(context.Background(), dest, c.reader, c.Size(), c.Mime())
	if err == nil {
		_ = c.Discard()
	}
	return err
}

// CopyTo copies the uploaded file bytes to a destination ID without removing the temp file.
func (c *UploadCtx) CopyTo(dest string) error {
	if c.backend == nil {
		b, err := storage.GetBackend("local")
		if err != nil {
			return err
		}
		c.backend = b
	}

	if err := c.Seek(0); err != nil {
		return err
	}

	return c.backend.Put(context.Background(), dest, c.reader, c.Size(), c.Mime())
}

// Discard cleans up the temporary upload file immediately.
func (c *UploadCtx) Discard() error {
	if c.discarded {
		return nil
	}
	c.discarded = true
	if c.reader != nil {
		_ = c.reader.Close()
	}
	if c.file != nil && c.file.TempPath != "" {
		return os.Remove(c.file.TempPath)
	}
	return nil
}

// Dedupe checks whether this file content already exists in the deduplication index.
func (c *UploadCtx) Dedupe() (existingID string, hit bool, err error) {
	sha := c.SHA256()
	if sha == "" {
		return "", false, fmt.Errorf("cannot dedupe without sha256")
	}
	id, ok := upload.GlobalDedup.Check(sha)
	return id, ok, nil
}

// DownloadCtx represents the download request context.
type DownloadCtx struct {
	fileID string
	params map[string]string
}

// NewDownloadCtx initializes a DownloadCtx.
func NewDownloadCtx(fileID string, params map[string]string) *DownloadCtx {
	return &DownloadCtx{
		fileID: fileID,
		params: params,
	}
}

// FileID returns the target file identifier to download.
func (c *DownloadCtx) FileID() string {
	return c.fileID
}

// Param returns a path or query parameter.
func (c *DownloadCtx) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}
