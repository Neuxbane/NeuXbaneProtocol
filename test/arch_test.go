package test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchABIImports verifies that nxp/abi imports ONLY standard library packages and nxp/errors.
func TestArchABIImports(t *testing.T) {
	abiDir := filepath.Join("..", "nxp", "abi")
	if _, err := os.Stat(abiDir); os.IsNotExist(err) {
		abiDir = filepath.Join("nxp", "abi")
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, abiDir, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse nxp/abi: %v", err)
	}

	allowedCustom := map[string]bool{
		`"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"`: true,
	}

	for pkgName, pkg := range pkgs {
		for fileName, f := range pkg.Files {
			if strings.HasSuffix(fileName, "_test.go") {
				continue
			}

			for _, imp := range f.Imports {
				importPath := imp.Path.Value
				cleanPath := strings.Trim(importPath, `"`)

				if allowedCustom[importPath] {
					continue
				}

				firstPart := strings.Split(cleanPath, "/")[0]
				if strings.Contains(firstPart, ".") || strings.HasPrefix(cleanPath, "github.com/Neuxbane/NeuXbaneProtocol/nxp/") {
					t.Errorf("[%s] %s: forbidden import %s in nxp/%s; nxp/abi may only import stdlib and nxp/errors",
						pkgName, filepath.Base(fileName), importPath, pkgName)
				}
			}
		}
	}
}

// TestArchErrorsImports verifies that nxp/errors imports ONLY standard library packages.
func TestArchErrorsImports(t *testing.T) {
	errorsDir := filepath.Join("..", "nxp", "errors")
	if _, err := os.Stat(errorsDir); os.IsNotExist(err) {
		errorsDir = filepath.Join("nxp", "errors")
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, errorsDir, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse nxp/errors: %v", err)
	}

	for pkgName, pkg := range pkgs {
		for fileName, f := range pkg.Files {
			if strings.HasSuffix(fileName, "_test.go") {
				continue
			}

			for _, imp := range f.Imports {
				importPath := imp.Path.Value
				cleanPath := strings.Trim(importPath, `"`)

				firstPart := strings.Split(cleanPath, "/")[0]
				if strings.Contains(firstPart, ".") || strings.HasPrefix(cleanPath, "nxp") {
					t.Errorf("[%s] %s: forbidden import %s in nxp/errors; nxp/errors may only import stdlib",
						pkgName, filepath.Base(fileName), importPath)
				}
			}
		}
	}
}

// TestArchDefineImports verifies that define/* imports ONLY nxp/* and standard library.
func TestArchDefineImports(t *testing.T) {
	defineDir := filepath.Join("..", "define")
	if _, err := os.Stat(defineDir); os.IsNotExist(err) {
		defineDir = "define"
	}

	fset := token.NewFileSet()
	err := filepath.Walk(defineDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("failed to parse %s: %v", path, err)
			return nil
		}

		for _, imp := range f.Imports {
			cleanPath := strings.Trim(imp.Path.Value, `"`)
			firstPart := strings.Split(cleanPath, "/")[0]

			// Is standard library?
			if !strings.Contains(firstPart, ".") {
				continue
			}

			// Must be nxp/*
			if strings.HasPrefix(cleanPath, "github.com/Neuxbane/NeuXbaneProtocol/nxp/") {
				continue
			}

			t.Errorf("%s: forbidden import %q; define/* may import ONLY nxp/* and stdlib", path, cleanPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk define directory: %v", err)
	}
}

// TestArchInternalNeverImportsDefine verifies internal/* never imports define/*.
func TestArchInternalNeverImportsDefine(t *testing.T) {
	internalDir := filepath.Join("..", "internal")
	if _, err := os.Stat(internalDir); os.IsNotExist(err) {
		internalDir = "internal"
	}

	fset := token.NewFileSet()
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// internal/generated is the only place allowed to import define/*
		if strings.Contains(filepath.ToSlash(path), "internal/generated") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("failed to parse %s: %v", path, err)
			return nil
		}

		for _, imp := range f.Imports {
			cleanPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(cleanPath, "NeuXbaneProtocol/define") {
				t.Errorf("%s: forbidden import %q; internal/* never imports define/*", path, cleanPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk internal directory: %v", err)
	}
}

// TestArchNxpNeverImportsCoreInternal verifies nxp/* never imports internal/runtime, internal/ipc, internal/router.
func TestArchNxpNeverImportsCoreInternal(t *testing.T) {
	nxpDir := filepath.Join("..", "nxp")
	if _, err := os.Stat(nxpDir); os.IsNotExist(err) {
		nxpDir = "nxp"
	}

	forbidden := []string{
		"NeuXbaneProtocol/internal/runtime",
		"NeuXbaneProtocol/internal/ipc",
		"NeuXbaneProtocol/internal/router",
	}

	fset := token.NewFileSet()
	err := filepath.Walk(nxpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("failed to parse %s: %v", path, err)
			return nil
		}

		for _, imp := range f.Imports {
			cleanPath := strings.Trim(imp.Path.Value, `"`)
			for _, forb := range forbidden {
				if strings.Contains(cleanPath, forb) {
					t.Errorf("%s: forbidden import %q; nxp/* must never import %s", path, cleanPath, forb)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk nxp directory: %v", err)
	}
}

// TestArchNoDependencyCycles verifies that internal/* package dependencies are strictly acyclic.
func TestArchNoDependencyCycles(t *testing.T) {
	internalDir := filepath.Join("..", "internal")
	if _, err := os.Stat(internalDir); os.IsNotExist(err) {
		internalDir = "internal"
	}

	const modPrefix = "github.com/Neuxbane/NeuXbaneProtocol/internal/"
	graph := make(map[string][]string)

	fset := token.NewFileSet()
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, _ := filepath.Rel(internalDir, filepath.Dir(path))
		pkgName := filepath.ToSlash(rel)

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil
		}

		for _, imp := range f.Imports {
			cleanPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(cleanPath, modPrefix) {
				depPkg := strings.TrimPrefix(cleanPath, modPrefix)
				if depPkg != pkgName {
					graph[pkgName] = append(graph[pkgName], depPkg)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk internal directory: %v", err)
	}

	// DFS cycle detection
	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=visited
	var pathStack []string

	var checkCycle func(u string) bool
	checkCycle = func(u string) bool {
		visited[u] = 1
		pathStack = append(pathStack, u)

		for _, v := range graph[u] {
			if visited[v] == 1 {
				t.Fatalf("detected package dependency cycle: %s -> %s", strings.Join(pathStack, " -> "), v)
				return true
			}
			if visited[v] == 0 {
				if checkCycle(v) {
					return true
				}
			}
		}

		pathStack = pathStack[:len(pathStack)-1]
		visited[u] = 2
		return false
	}

	for pkg := range graph {
		if visited[pkg] == 0 {
			checkCycle(pkg)
		}
	}
}
