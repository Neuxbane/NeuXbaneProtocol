// Package upload answers: how are incoming file uploads received, verified, deduplicated, and quarantined?
package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ReceivedFile holds metadata and local storage for an ingested upload.
type ReceivedFile struct {
	OriginalName string
	SafeName     string
	Mime         string
	SHA256       string
	Size         int64
	TempPath     string
	Quarantined  bool
}

// Receive streams data from r to a temporary file while computing sha256, sniffing mime, and verifying quota.
func Receive(r io.Reader, originalName string, maxQuota int64) (*ReceivedFile, error) {
	tmpFile, err := os.CreateTemp("", "nxp-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp upload file: %w", err)
	}
	defer tmpFile.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(tmpFile, hasher)

	// Read initial 512 bytes for MIME detection
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(r, sniffBuf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("sniff mime: %w", err)
	}

	mime := http.DetectContentType(sniffBuf[:n])
	if _, err := mw.Write(sniffBuf[:n]); err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("write sniffed header: %w", err)
	}

	written, err := io.Copy(mw, r)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("stream upload: %w", err)
	}

	totalSize := int64(n) + written
	if maxQuota > 0 && totalSize > maxQuota {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("upload size %d exceeds quota %d", totalSize, maxQuota)
	}

	sha := hex.EncodeToString(hasher.Sum(nil))
	safeName := SanitizeFilename(originalName)

	return &ReceivedFile{
		OriginalName: originalName,
		SafeName:     safeName,
		Mime:         mime,
		SHA256:       sha,
		Size:         totalSize,
		TempPath:     tmpFile.Name(),
	}, nil
}

// SanitizeFilename normalizes unicode characters and eliminates path traversals and illegal symbols.
func SanitizeFilename(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "\\", "/")
	base = filepath.Base(base)

	var sb strings.Builder
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else if unicode.IsSpace(r) {
			sb.WriteRune('_')
		}
	}

	res := sb.String()
	if res == "" || res == "." {
		return "unnamed_file"
	}
	return res
}

// ReceiveFromURL fetches a file from an HTTP/HTTPS URL and streams it to a temporary file.
func ReceiveFromURL(ctx context.Context, fileURL string, maxQuota int64) (*ReceivedFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create url request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch url returned status %d", resp.StatusCode)
	}

	filename := "file.bin"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if strings.Contains(cd, "filename=") {
			parts := strings.Split(cd, "filename=")
			if len(parts) > 1 {
				clean := strings.Trim(strings.Split(parts[1], ";")[0], "\" ")
				if clean != "" {
					filename = clean
				}
			}
		}
	} else {
		if parsed, err := url.Parse(fileURL); err == nil {
			base := filepath.Base(parsed.Path)
			if base != "" && base != "/" && base != "." {
				filename = base
			}
		}
	}

	return Receive(resp.Body, filename, maxQuota)
}
