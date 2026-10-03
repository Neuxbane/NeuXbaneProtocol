package upload_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/upload"
)

func TestReceiveAndSanitize(t *testing.T) {
	data := []byte("%PDF-1.4 mock pdf payload bytes 1234567890")
	rec, err := upload.Receive(bytes.NewReader(data), "../../dangerous/file:name*?.pdf", 1024*1024)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	defer os.Remove(rec.TempPath)

	if rec.SafeName == "" || strings.Contains(rec.SafeName, "..") {
		t.Errorf("expected sanitized safe name, got %q", rec.SafeName)
	}
	if rec.Size != int64(len(data)) {
		t.Errorf("size mismatch: got %d, want %d", rec.Size, len(data))
	}
	if rec.SHA256 == "" {
		t.Errorf("expected non-empty sha256")
	}
}

func TestQuarantinePolicies(t *testing.T) {
	if !upload.ShouldQuarantine("malware.exe", "application/x-dosexec") {
		t.Errorf("expected .exe to be quarantined")
	}
	if !upload.ShouldQuarantine("script.sh", "text/x-shellscript") {
		t.Errorf("expected .sh to be quarantined")
	}
	if upload.ShouldQuarantine("photo.jpg", "image/jpeg") {
		t.Errorf("jpg should not be quarantined")
	}
}

func TestDedup(t *testing.T) {
	dedup := upload.NewDedupIndex()
	sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	_, hit := dedup.Check(sha)
	if hit {
		t.Errorf("expected dedup miss")
	}

	dedup.Register(sha, "file-123")
	id, hit := dedup.Check(sha)
	if !hit || id != "file-123" {
		t.Errorf("expected dedup hit for file-123, got %s", id)
	}
}

func TestResumeSessions(t *testing.T) {
	mgr := upload.GlobalSessions
	session, err := mgr.CreateSession("bigfile.dat", 1000)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	token := session.Token
	chunk1 := []byte("chunk-1-data-")
	n1, err := mgr.AppendChunk(token, bytes.NewReader(chunk1))
	if err != nil || n1 != int64(len(chunk1)) {
		t.Fatalf("append chunk 1 failed: %v", err)
	}

	chunk2 := []byte("chunk-2-data")
	n2, err := mgr.AppendChunk(token, bytes.NewReader(chunk2))
	if err != nil || n2 != int64(len(chunk1)+len(chunk2)) {
		t.Fatalf("append chunk 2 failed: %v", err)
	}

	completedFile, err := mgr.Complete(token)
	if err != nil || completedFile == "" {
		t.Fatalf("complete session failed: %v", err)
	}
}
