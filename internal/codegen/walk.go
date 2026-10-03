package codegen

import (
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
}

// ComputeRoutePath transforms a relative define file path to an API route path according to framework rules:
// - Strip define/ prefix
// - Strip .go suffix
// - Split on /
// - Drop trailing "index"
// - Wrap [x] -> {x}
// - Prepend /
func ComputeRoutePath(relPath string) string {
	clean := filepath.ToSlash(relPath)
	clean = strings.TrimPrefix(clean, "define/")
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimSuffix(clean, ".go")

	segments := strings.Split(clean, "/")
	outSegments := make([]string, 0, len(segments))

	for i, seg := range segments {
		// Strip transport/method prefix from the filename segment (last segment)
		if i == len(segments)-1 {
			seg = stripMethodPrefix(seg)
		}

		// Drop trailing "index"
		if i == len(segments)-1 && seg == "index" {
			continue
		}

		// Convert [param] to {param}
		if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
			seg = "{" + seg[1:len(seg)-1] + "}"
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

// InferTransportAndMethod extracts default transport and method from filename prefixes.
func InferTransportAndMethod(fileName string) (abi.Transport, string) {
	base := strings.TrimSuffix(fileName, ".go")

	switch {
	case strings.HasPrefix(base, "get_"):
		return abi.TransportREST, "GET"
	case strings.HasPrefix(base, "post_"):
		return abi.TransportREST, "POST"
	case strings.HasPrefix(base, "put_"):
		return abi.TransportREST, "PUT"
	case strings.HasPrefix(base, "del_"):
		return abi.TransportREST, "DELETE"
	case strings.HasPrefix(base, "delete_"):
		return abi.TransportREST, "DELETE"
	case strings.HasPrefix(base, "patch_"):
		return abi.TransportREST, "PATCH"
	case strings.HasPrefix(base, "ws_"):
		return abi.TransportWebSocket, "CONNECT"
	case strings.HasPrefix(base, "grpc_"):
		return abi.TransportGRPC, "POST"
	case strings.HasPrefix(base, "udp_"):
		return abi.TransportUDP, "SEND"
	case strings.HasPrefix(base, "mqtt_"):
		return abi.TransportMQTT, "SUB"
	case strings.HasPrefix(base, "nats_"):
		return abi.TransportNATS, "SUB"
	case strings.HasPrefix(base, "kafka_"):
		return abi.TransportKafka, "CONSUME"
	case base == "index":
		return abi.TransportREST, "GET"
	default:
		return abi.TransportREST, "GET"
	}
}

func stripMethodPrefix(seg string) string {
	prefixes := []string{
		"get_", "post_", "put_", "del_", "delete_", "patch_",
		"ws_", "grpc_", "udp_", "mqtt_", "nats_", "kafka_",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(seg, p) {
			return strings.TrimPrefix(seg, p)
		}
	}
	return seg
}

// WalkDefineTree discovers all route handler Go files under defineDir.
func WalkDefineTree(defineDir string) ([]FileRoute, error) {
	var routes []FileRoute

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

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(contentBytes)

		rel, _ := filepath.Rel(defineDir, path)
		directives := ParseDirectives(content)

		inferredTransport, inferredMethod := InferTransportAndMethod(filepath.Base(path))

		routePath := ComputeRoutePath("define/" + rel)
		if directives.Route != "" {
			routePath = directives.Route
		}

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
		})

		return nil
	})

	return routes, err
}
