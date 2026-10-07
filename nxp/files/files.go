// Package files provides developer-facing file upload, download, and storage primitives.
package files

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

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

// NewUploadCtxFromURL downloads a file from an HTTP/HTTPS URL and initializes an UploadCtx.
func NewUploadCtxFromURL(ctx context.Context, fileURL string, fields map[string]string, backend storage.Backend) (*UploadCtx, error) {
	rec, err := upload.ReceiveFromURL(ctx, fileURL, 0)
	if err != nil {
		return nil, err
	}
	return NewUploadCtx(rec, fields, backend)
}

// InspectResult contains byte-range inspection output and metadata.
type InspectResult struct {
	Source    string `json:"source"`
	From      int64  `json:"from"`
	Length    int64  `json:"length"`
	ReadBytes int64  `json:"read_bytes"`
	TotalSize int64  `json:"total_size"`
	Data      []byte `json:"-"`
	DataHex   string `json:"data_hex,omitempty"`
	DataText  string `json:"data_text,omitempty"`
	IsText    bool   `json:"is_text"`
	EOF       bool   `json:"eof"`
}

// Inspect reads a byte slice from source (HTTP/HTTPS URL, NXP storage backend, or local path) between [from, from+length).
func Inspect(ctx context.Context, source string, from, length int64, backend storage.Backend) (*InspectResult, error) {
	if length <= 0 {
		length = 1024
	}
	if from < 0 {
		from = 0
	}

	res := &InspectResult{
		Source: source,
		From:   from,
		Length: length,
	}

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, fmt.Errorf("create inspect request: %w", err)
		}
		end := from + length - 1
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, end))

		client := http.DefaultClient
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("inspect http fetch: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusPartialContent {
			if cr := resp.Header.Get("Content-Range"); cr != "" {
				parts := strings.Split(cr, "/")
				if len(parts) == 2 && parts[1] != "*" {
					if total, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						res.TotalSize = total
					}
				}
			}
			buf, err := io.ReadAll(io.LimitReader(resp.Body, length))
			if err != nil {
				return nil, fmt.Errorf("read inspect response: %w", err)
			}
			res.Data = buf
			res.ReadBytes = int64(len(buf))
		} else if resp.StatusCode == http.StatusOK {
			if resp.ContentLength > 0 {
				res.TotalSize = resp.ContentLength
			}
			if from > 0 {
				if _, err := io.CopyN(io.Discard, resp.Body, from); err != nil && err != io.EOF {
					return nil, fmt.Errorf("skip to offset: %w", err)
				}
			}
			buf, err := io.ReadAll(io.LimitReader(resp.Body, length))
			if err != nil {
				return nil, fmt.Errorf("read inspect body: %w", err)
			}
			res.Data = buf
			res.ReadBytes = int64(len(buf))
		} else {
			return nil, fmt.Errorf("inspect http returned status %d", resp.StatusCode)
		}

		if res.TotalSize > 0 && res.From+res.ReadBytes >= res.TotalSize {
			res.EOF = true
		}
	} else {
		var rc io.ReadCloser
		var total int64

		if backend != nil {
			if rRange, errRange := backend.GetRange(ctx, source, from, length); errRange == nil {
				rc = rRange
				if _, sz, errGet := backend.Get(ctx, source); errGet == nil {
					total = sz
				}
			} else if rAll, sz, errGet := backend.Get(ctx, source); errGet == nil {
				total = sz
				if from > 0 {
					_, _ = io.CopyN(io.Discard, rAll, from)
				}
				rc = rAll
			}
		}

		if rc == nil {
			f, errFile := os.Open(source)
			if errFile != nil {
				return nil, fmt.Errorf("source not found in storage or disk: %q", source)
			}
			fi, _ := f.Stat()
			if fi != nil {
				total = fi.Size()
			}
			if from > 0 {
				if _, err := f.Seek(from, io.SeekStart); err != nil {
					_ = f.Close()
					return nil, fmt.Errorf("seek to offset %d: %w", from, err)
				}
			}
			rc = f
		}

		defer rc.Close()
		res.TotalSize = total
		buf, err := io.ReadAll(io.LimitReader(rc, length))
		if err != nil {
			return nil, fmt.Errorf("read inspect data: %w", err)
		}
		res.Data = buf
		res.ReadBytes = int64(len(buf))
		if res.TotalSize > 0 && res.From+res.ReadBytes >= res.TotalSize {
			res.EOF = true
		}
	}

	if utf8.Valid(res.Data) {
		res.IsText = true
		res.DataText = string(res.Data)
	} else {
		res.IsText = false
		res.DataHex = hex.EncodeToString(res.Data)
	}

	return res, nil
}
