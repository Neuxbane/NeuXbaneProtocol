package files_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/storage"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/upload"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/files"
)

func TestUploadCtxLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "files-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend := storage.NewLocalBackend(filepath.Join(tmpDir, "storage"))

	data := []byte("upload content for testing")
	rec, err := upload.Receive(bytes.NewReader(data), "document.txt", 1024*1024)
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}

	fields := map[string]string{
		"category": "reports",
	}

	ctx, err := files.NewUploadCtx(rec, fields, backend)
	if err != nil {
		t.Fatalf("NewUploadCtx failed: %v", err)
	}
	defer ctx.Discard()

	if ctx.OriginalName() != "document.txt" {
		t.Errorf("expected document.txt, got %s", ctx.OriginalName())
	}
	if ctx.Size() != int64(len(data)) {
		t.Errorf("size mismatch: got %d, want %d", ctx.Size(), len(data))
	}
	if ctx.Field("category") != "reports" {
		t.Errorf("field category mismatch: got %s", ctx.Field("category"))
	}
	if ctx.Quarantine() {
		t.Errorf("txt file should not be quarantined")
	}

	// Test CopyTo
	if err := ctx.CopyTo("doc-copy-1"); err != nil {
		t.Fatalf("CopyTo failed: %v", err)
	}
	exists, err := backend.Exists(nil, "doc-copy-1")
	if err != nil || !exists {
		t.Fatalf("expected doc-copy-1 to exist in backend")
	}

	// Test MoveTo
	if err := ctx.MoveTo("doc-final-1"); err != nil {
		t.Fatalf("MoveTo failed: %v", err)
	}
	existsFinal, err := backend.Exists(nil, "doc-final-1")
	if err != nil || !existsFinal {
		t.Fatalf("expected doc-final-1 to exist in backend")
	}

	// DownloadCtx
	dlCtx := files.NewDownloadCtx("doc-final-1", map[string]string{"v": "2"})
	if dlCtx.FileID() != "doc-final-1" || dlCtx.Param("v") != "2" {
		t.Errorf("DownloadCtx mismatch: %+v", dlCtx)
	}
}

func TestInspectLocalFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "inspect-test-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	tmpFile.Close()

	res, err := files.Inspect(context.Background(), tmpFile.Name(), 10, 5, nil)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	if res.From != 10 || res.Length != 5 {
		t.Errorf("unexpected bounds: from=%d length=%d", res.From, res.Length)
	}
	if res.ReadBytes != 5 {
		t.Errorf("expected 5 bytes read, got %d", res.ReadBytes)
	}
	if string(res.Data) != "ABCDE" {
		t.Errorf("expected ABCDE, got %s", string(res.Data))
	}
	if !res.IsText || res.DataText != "ABCDE" {
		t.Errorf("expected text ABCDE, got %v / %s", res.IsText, res.DataText)
	}
	if res.TotalSize != int64(len(content)) {
		t.Errorf("expected total size %d, got %d", len(content), res.TotalSize)
	}
}

func TestInspectHTTP(t *testing.T) {
	payload := "HELLO_WORLD_HTTP_RANGE_INSPECT"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "test.txt", time.Time{}, bytes.NewReader([]byte(payload)))
	}))
	defer ts.Close()

	// Inspect bytes 6 to 11 ("WORLD")
	res, err := files.Inspect(context.Background(), ts.URL+"/test.txt", 6, 5, nil)
	if err != nil {
		t.Fatalf("Inspect HTTP failed: %v", err)
	}

	if string(res.Data) != "WORLD" {
		t.Errorf("expected WORLD, got %s", string(res.Data))
	}
	if res.ReadBytes != 5 {
		t.Errorf("expected 5 read bytes, got %d", res.ReadBytes)
	}
}

func TestNewUploadCtxFromURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", `attachment; filename="remotefile.txt"`)
		_, _ = fmt.Fprint(w, "streamed-from-remote-url")
	}))
	defer ts.Close()

	ctx, err := files.NewUploadCtxFromURL(context.Background(), ts.URL+"/download", map[string]string{"foo": "bar"}, nil)
	if err != nil {
		t.Fatalf("NewUploadCtxFromURL failed: %v", err)
	}
	defer ctx.Discard()

	if ctx.OriginalName() != "remotefile.txt" {
		t.Errorf("expected remotefile.txt, got %s", ctx.OriginalName())
	}
	if ctx.Field("foo") != "bar" {
		t.Errorf("expected field foo=bar, got %s", ctx.Field("foo"))
	}
	if ctx.Size() != int64(len("streamed-from-remote-url")) {
		t.Errorf("expected size %d, got %d", len("streamed-from-remote-url"), ctx.Size())
	}
}
