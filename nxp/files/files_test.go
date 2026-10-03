package files_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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
