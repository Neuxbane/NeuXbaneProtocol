package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StageDefineTree copies the authoring define/ tree into a Go-compilable
// staging directory.
//
// Handler authors write dynamic route segments with an "@" prefix, e.g.
// define/agents/@id/get_index.go -> GET /agents/{id}. Go import paths reject
// "@" (and "[", "{"), so the tree cannot be compiled in place. Staging renames
// every "@param" path segment to "_param" and rewrites the package clause of
// each copied file to match its (sanitized) directory name.
//
// It returns a map from the original absolute file path to the staged absolute
// file path so callers can resolve import paths against the staged tree.
func StageDefineTree(defineDir, stageDir string) (map[string]string, error) {
	if err := os.RemoveAll(stageDir); err != nil {
		return nil, fmt.Errorf("clean stage dir: %w", err)
	}

	origToStaged := make(map[string]string)

	err := filepath.Walk(defineDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if info.IsDir() {
			if strings.HasPrefix(base, ".") && path != defineDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(defineDir, path)
		if err != nil {
			return err
		}

		stagedRel := sanitizeStagePath(rel)
		stagedPath := filepath.Join(stageDir, stagedRel)

		if err := os.MkdirAll(filepath.Dir(stagedPath), 0755); err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		pkgName := sanitizePackageName(filepath.Base(filepath.Dir(stagedPath)))
		rewritten := rewritePackageClause(string(content), pkgName)

		if err := os.WriteFile(stagedPath, []byte(rewritten), 0644); err != nil {
			return err
		}

		absOrig, _ := filepath.Abs(path)
		absStaged, _ := filepath.Abs(stagedPath)
		origToStaged[absOrig] = absStaged
		return nil
	})
	if err != nil {
		return nil, err
	}

	return origToStaged, nil
}

// sanitizeStagePath rewrites each path segment of a relative define path so it
// is a valid Go import path component. "@param" becomes "_param".
func sanitizeStagePath(rel string) string {
	rel = filepath.ToSlash(rel)
	segments := strings.Split(rel, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, "@") && len(seg) > 1 {
			segments[i] = "_" + seg[1:]
		}
	}
	return filepath.Join(segments...)
}

// sanitizePackageName turns a directory name into a valid Go package
// identifier. "_id" -> "id", "app.css" -> "appcss", "my-dir" -> "mydir".
func sanitizePackageName(dir string) string {
	var b strings.Builder
	for _, r := range dir {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" {
		return "handler"
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "pkg" + name
	}
	return name
}

// rewritePackageClause replaces the leading "package <name>" clause of a Go
// source file with "package <pkgName>".
func rewritePackageClause(src, pkgName string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			lines[i] = "package " + pkgName
			break
		}
	}
	return strings.Join(lines, "\n")
}
