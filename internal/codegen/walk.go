package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// FileRoute contains the inferred route properties for a file under define/.
type FileRoute struct {
	FilePath    string
	PackagePath string
	PackageName string
	Path        string
	Method      string
	Transport   abi.Transport
	Directives  *Directives
	Content     string
	// Guards lists the guard handler IDs inherited from ancestor index.go
	// files, ordered from the outermost folder to the innermost.
	Guards []string
}

// ComputeRoutePath transforms a relative define file path to an API route path
// according to the bare-method-file convention:
//
//   - Folders are path segments. "@x" (or staged "_x") -> "{x}".
//   - A bare method filename (get.go, post.go, ...) is dropped from the path.
//   - "index.go" is a guard implementation, never a route.
//
// Examples:
//
//	define/get.go                    -> /
//	define/agents/get.go             -> /agents
//	define/agents/@id/get.go         -> /agents/{id}
//	define/agents/@id/patch.go       -> /agents/{id}
//	define/chat/room/ws.go           -> /chat/room
//	define/sensors/telemetry/mqtt.go -> /sensors/telemetry
func ComputeRoutePath(relPath string) string {
	clean := filepath.ToSlash(relPath)
	clean = strings.TrimPrefix(clean, "define/")
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimSuffix(clean, ".go")

	segments := strings.Split(clean, "/")
	outSegments := make([]string, 0, len(segments))

	for i, seg := range segments {
		last := i == len(segments)-1

		if last {
			// The bare method filename never contributes to the path.
			if isMethodSegment(seg) || seg == "index" {
				continue
			}
		}

		// Convert [param] to {param}
		if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
			seg = "{" + seg[1:len(seg)-1] + "}"
		}

		// Convert @param to {param}. The "@" form is the authoring syntax on
		// disk; it is staged to "_param" before compilation because Go import
		// paths reject "@". Both forms map to a dynamic route segment.
		if strings.HasPrefix(seg, "@") && len(seg) > 1 {
			seg = "{" + seg[1:] + "}"
		}
		if strings.HasPrefix(seg, "_") && len(seg) > 1 {
			seg = "{" + seg[1:] + "}"
		}

		if seg != "" {
			outSegments = append(outSegments, seg)
		}
	}

	if len(outSegments) == 0 {
		return "/"
	}
	return "/" + strings.Join(outSegments, "/")
}

// isMethodSegment reports whether a path segment is a bare method name.
func isMethodSegment(seg string) bool {
	switch seg {
	case "get", "post", "put", "patch", "delete", "del",
		"ws", "grpc", "udp", "mqtt", "nats", "kafka", "rtc":
		return true
	}
	return false
}

// methodFromSegment maps a bare method segment to its transport and method.
func methodFromSegment(seg string) (abi.Transport, string, bool) {
	switch seg {
	case "get":
		return abi.TransportREST, "GET", true
	case "post":
		return abi.TransportREST, "POST", true
	case "put":
		return abi.TransportREST, "PUT", true
	case "patch":
		return abi.TransportREST, "PATCH", true
	case "delete", "del":
		return abi.TransportREST, "DELETE", true
	case "ws":
		return abi.TransportWebSocket, "CONNECT", true
	case "rtc":
		return abi.TransportWebRTC, "CONNECT", true
	case "grpc":
		return abi.TransportGRPC, "POST", true
	case "udp":
		return abi.TransportUDP, "SEND", true
	case "mqtt":
		return abi.TransportMQTT, "SUB", true
	case "nats":
		return abi.TransportNATS, "SUB", true
	case "kafka":
		return abi.TransportKafka, "CONSUME", true
	}
	return "", "", false
}

// InferTransportAndMethod extracts the transport and method for a bare method file.
// The method is taken from the filename itself (get.go -> GET).
//
// relPath is the path relative to define/ (e.g. "agents/@id/get.go").
func InferTransportAndMethod(relPath string) (abi.Transport, string) {
	clean := filepath.ToSlash(relPath)
	clean = strings.TrimPrefix(clean, "define/")
	clean = strings.TrimPrefix(clean, "/")
	segments := strings.Split(clean, "/")

	base := strings.TrimSuffix(segments[len(segments)-1], ".go")
	if tr, m, ok := methodFromSegment(base); ok {
		return tr, m
	}

	return abi.TransportREST, "GET"
}

// IsGuardFile reports whether a filename is a guard implementation (index.go).
// Guard files declare a Guard function and are never routes themselves.
func IsGuardFile(fileName string) bool {
	return strings.TrimSuffix(fileName, ".go") == "index"
}

// WalkDefineTree discovers all route handler Go files under defineDir.
func WalkDefineTree(defineDir string) ([]FileRoute, error) {
	var routes []FileRoute

	// Pass 1: collect guard implementations. Every index.go implements a guard
	// named after its folder path relative to define/ (e.g. define/auth/index.go
	// implements guard "auth"; define/user/admin/index.go implements
	// "user/admin").
	guardsByName := make(map[string]string) // guard name -> guard handler ID
	_ = filepath.Walk(defineDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !IsGuardFile(filepath.Base(path)) {
			return nil
		}
		dir := filepath.Dir(path)
		name := guardName(defineDir, dir)
		guardsByName[name] = guardHandlerID(defineDir, dir)
		return nil
	})

	err := filepath.Walk(defineDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// index.go files are folder guards, not routes.
		if IsGuardFile(filepath.Base(path)) {
			return nil
		}

		base := filepath.Base(path)
		methodName := strings.TrimSuffix(base, ".go")
		rel, _ := filepath.Rel(defineDir, path)
		if !isMethodSegment(methodName) {
			return fmt.Errorf("route %s: unsupported handler filename %q: NXP only supports bare method files (e.g. define/agents/get.go); method folders (handler.go) and legacy prefix files are not supported", rel, base)
		}

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(contentBytes)

		directives := ParseDirectives(content)

		if !directives.HasGuardDirective {
			return fmt.Errorf("route %s: missing required @guard directive (use '@guard <name>', '@guard <g1> <g2>', or '@guard' for none)", rel)
		}

		resolvedGuards, err := resolveGuards(directives.Guards, guardsByName)
		if err != nil {
			return fmt.Errorf("route %s: %w", rel, err)
		}

		inferredTransport, inferredMethod := InferTransportAndMethod(rel)

		routePath := ComputeRoutePath("define/" + rel)

		transport := inferredTransport
		if directives.Transport != "" {
			transport = directives.Transport
		}

		method := inferredMethod
		if directives.Method != "" {
			method = directives.Method
		}

		pkgDir := filepath.Dir(path)
		pkgName := filepath.Base(pkgDir)

		routes = append(routes, FileRoute{
			FilePath:    path,
			PackagePath: pkgDir,
			PackageName: pkgName,
			Path:        routePath,
			Method:      method,
			Transport:   transport,
			Directives:  directives,
			Content:     content,
			Guards:      resolvedGuards,
		})

		return nil
	})

	return routes, err
}

// resolveGuards maps guard names declared via @guard to their guard handler IDs.
// Returns an error if any declared guard does not match an existing index.go guard.
func resolveGuards(names []string, guardsByName map[string]string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n == "none" || n == "public" {
			continue
		}
		if id, ok := guardsByName[n]; ok {
			out = append(out, id)
			continue
		}
		sanitized := strings.ReplaceAll(n, "@", "_")
		if id, ok := guardsByName[sanitized]; ok {
			out = append(out, id)
			continue
		}
		if strings.HasPrefix(n, "guard.") {
			trimmed := strings.TrimPrefix(n, "guard.")
			trimmedPath := strings.ReplaceAll(trimmed, ".", "/")
			if id, ok := guardsByName[trimmedPath]; ok {
				out = append(out, id)
				continue
			}
			if id, ok := guardsByName[trimmed]; ok {
				out = append(out, id)
				continue
			}
		}
		return nil, fmt.Errorf("unknown guard %q (no matching index.go found in define/%s)", n, n)
	}
	return out, nil
}

// guardName derives a guard's logical name from its folder path relative to
// define/ (e.g. define/auth -> "auth", define/user/admin -> "user/admin").
func guardName(defineDir, dir string) string {
	rel, _ := filepath.Rel(defineDir, dir)
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return "index"
	}
	return rel
}

// GuardFile describes a folder guard (index.go) discovered under define/.
type GuardFile struct {
	// ID is the guard handler ID (e.g. "guard.agents").
	ID string
	// FilePath is the absolute path to the index.go file.
	FilePath string
	// PackagePath is the absolute directory containing the guard.
	PackagePath string
	// PackageName is the Go package name of the guard's directory.
	PackageName string
	// Content is the raw source of the guard file.
	Content string
}

// WalkGuards discovers every folder guard (index.go) under defineDir.
func WalkGuards(defineDir string) ([]GuardFile, error) {
	var guards []GuardFile
	err := filepath.Walk(defineDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsGuardFile(filepath.Base(path)) {
			return nil
		}
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		guards = append(guards, GuardFile{
			ID:          guardHandlerID(defineDir, dir),
			FilePath:    path,
			PackagePath: dir,
			PackageName: filepath.Base(dir),
			Content:     string(contentBytes),
		})
		return nil
	})
	return guards, err
}

// guardHandlerID derives the handler ID for a guard implementation from its
// directory (e.g. define/auth -> "guard.auth", define/user/admin ->
// "guard.user.admin").
func guardHandlerID(defineDir, dir string) string {
	rel, _ := filepath.Rel(defineDir, dir)
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return "guard.index"
	}
	id := strings.ReplaceAll(rel, "/", ".")
	return "guard." + id
}
