package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ComputeBuildID computes a deterministic SHA256 checksum over all .go files under defineDir.
func ComputeBuildID(defineDir string) (string, error) {
	hasher := sha256.New()

	var files []string
	err := filepath.Walk(defineDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)

	for _, f := range files {
		hasher.Write([]byte(f))
		fileBytes, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		hasher.Write(fileBytes)
	}

	return hex.EncodeToString(hasher.Sum(nil))[:16], nil
}

// WriteBuildIDFile saves the build ID to .nxp-build-id.
func WriteBuildIDFile(targetPath, buildID string) error {
	return os.WriteFile(targetPath, []byte(buildID+"\n"), 0644)
}

// ReadBuildIDFile reads the build ID from .nxp-build-id.
func ReadBuildIDFile(targetPath string) (string, error) {
	b, err := os.ReadFile(targetPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// ChecksumReader streams sha256 for a reader.
func ChecksumReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
