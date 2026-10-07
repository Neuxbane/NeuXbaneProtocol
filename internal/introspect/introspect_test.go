package introspect_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/introspect"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
)

func setupTestTable() *router.Table {
	tbl := router.NewTable()

	// / (root index)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "root.index",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/",
		},
	})

	// /auth (index.go -> /auth, appears as self)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.index",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth",
			Auth:      "required",
			RateLimit: &abi.RateLimitConfig{RPS: 20, Burst: 40},
			Shape: abi.RequestResponseShape{
				Responses: map[int]*abi.Schema{
					200: {Type: "object"},
				},
			},
		},
	})

	// /auth/profile (GET and PATCH)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.profile.get",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth/profile",
			Auth:      "required",
			Shape: abi.RequestResponseShape{
				Responses: map[int]*abi.Schema{
					200: {Type: "object"},
				},
			},
		},
	})
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.profile.patch",
			Transport: abi.TransportREST,
			Method:    "PATCH",
			Path:      "/auth/profile",
			Auth:      "required",
			Shape: abi.RequestResponseShape{
				Request: &abi.Schema{Type: "object"},
				Responses: map[int]*abi.Schema{
					200: {Type: "object"},
				},
			},
		},
	})

	// /auth/{id} (dynamic branch)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.tokens.get",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth/{id}/tokens",
		},
	})

	return tbl
}

func TestIntrospectJSON(t *testing.T) {
	tbl := setupTestTable()
	cfg := config.DefaultConfig().Introspect
	cfg.Public = true
	interceptor := introspect.NewInterceptor(cfg, "bld-test-123")

	// 1. GET /?nxp
	req := httptest.NewRequest("GET", "/?nxp", nil)
	rec := httptest.NewRecorder()
	handled := interceptor.Intercept(rec, req, tbl)
	if !handled {
		t.Fatalf("expected ?nxp request to be intercepted")
	}

	var rootResp introspect.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &rootResp); err != nil {
		t.Fatalf("unmarshal root resp failed: %v", err)
	}
	if rootResp.Path != "/" {
		t.Errorf("expected path /, got %s", rootResp.Path)
	}
	if rootResp.Self == nil || rootResp.Self.Handler != "root.index" {
		t.Errorf("expected root self handler, got %+v", rootResp.Self)
	}

	// 2. GET /auth?nxp
	reqAuth := httptest.NewRequest("GET", "/auth?nxp", nil)
	recAuth := httptest.NewRecorder()
	interceptor.Intercept(recAuth, reqAuth, tbl)

	var authResp introspect.Response
	if err := json.Unmarshal(recAuth.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("unmarshal auth resp failed: %v", err)
	}

	// Rule: index.go appears as self, never as child named index
	if authResp.Self == nil || authResp.Self.Handler != "auth.index" {
		t.Fatalf("expected self auth.index, got %+v", authResp.Self)
	}
	for _, c := range authResp.Children {
		if c.Name == "index" {
			t.Errorf("violation: 'index' appeared as a child!")
		}
	}

	// Rule: dynamic segments render as {id} with dynamic: true
	var foundDynamic bool
	for _, c := range authResp.Children {
		if c.Name == "{id}" && c.Dynamic {
			foundDynamic = true
			if c.Kind != "branch" {
				t.Errorf("expected {id} to be a branch")
			}
		}
	}
	if !foundDynamic {
		t.Errorf("expected dynamic child {id} under /auth")
	}
}

func TestIntrospectSchemaPerMethod(t *testing.T) {
	tbl := setupTestTable()
	cfg := config.DefaultConfig().Introspect
	cfg.Public = true
	interceptor := introspect.NewInterceptor(cfg, "bld-test-schemas")

	req := httptest.NewRequest("GET", "/auth/profile?nxp", nil)
	rec := httptest.NewRecorder()
	interceptor.Intercept(rec, req, tbl)

	var resp introspect.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp failed: %v", err)
	}
	if resp.Self == nil {
		t.Fatalf("expected self view for /auth/profile")
	}

	// The schema contract is keyed by method: both GET and PATCH must be present.
	if len(resp.Self.Schemas) != 2 {
		t.Fatalf("expected 2 method schemas, got %d: %+v", len(resp.Self.Schemas), resp.Self.Schemas)
	}
	if _, ok := resp.Self.Schemas["GET"]; !ok {
		t.Errorf("expected schema for GET")
	}
	if _, ok := resp.Self.Schemas["PATCH"]; !ok {
		t.Errorf("expected schema for PATCH")
	}
}

func TestIntrospectContentNegotiation(t *testing.T) {
	tbl := setupTestTable()
	cfg := config.DefaultConfig().Introspect
	cfg.Public = true
	interceptor := introspect.NewInterceptor(cfg, "bld-test-123")

	// 1. Accept: text/plain -> Tree format
	reqTree := httptest.NewRequest("GET", "/auth?nxp", nil)
	reqTree.Header.Set("Accept", "text/plain")
	recTree := httptest.NewRecorder()
	interceptor.Intercept(recTree, reqTree, tbl)

	treeOutput := recTree.Body.String()
	if !strings.Contains(treeOutput, "/auth") || !strings.Contains(treeOutput, "profile") {
		t.Errorf("unexpected tree output:\n%s", treeOutput)
	}

	// 2. Accept: text/html -> Browser page
	reqHTML := httptest.NewRequest("GET", "/auth?nxp", nil)
	reqHTML.Header.Set("Accept", "text/html")
	recHTML := httptest.NewRecorder()
	interceptor.Intercept(recHTML, reqHTML, tbl)

	htmlOutput := recHTML.Body.String()
	if !strings.Contains(htmlOutput, "<!DOCTYPE html>") || !strings.Contains(htmlOutput, "profile") {
		t.Errorf("unexpected HTML output:\n%s", htmlOutput)
	}

	// 3. Accept: application/openapi+json -> OpenAPI 3.1
	reqOpenAPI := httptest.NewRequest("GET", "/auth?nxp", nil)
	reqOpenAPI.Header.Set("Accept", "application/openapi+json")
	recOpenAPI := httptest.NewRecorder()
	interceptor.Intercept(recOpenAPI, reqOpenAPI, tbl)

	var openAPIDoc map[string]any
	if err := json.Unmarshal(recOpenAPI.Body.Bytes(), &openAPIDoc); err != nil {
		t.Fatalf("unmarshal OpenAPI failed: %v", err)
	}
	if openAPIDoc["openapi"] != "3.1.0" {
		t.Errorf("expected OpenAPI 3.1.0, got %v", openAPIDoc["openapi"])
	}

	// 4. Schema Bundle companion endpoint: GET /__nxp/schema?nxp&build=xxx
	reqBundle := httptest.NewRequest("GET", "/__nxp/schema?nxp&build=bld-test-123", nil)
	recBundle := httptest.NewRecorder()
	interceptor.Intercept(recBundle, reqBundle, tbl)

	if recBundle.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("expected immutable cache header for schema bundle")
	}
	if !strings.Contains(recBundle.Body.String(), "definitions") {
		t.Errorf("expected definitions in schema bundle")
	}
}

func TestIntrospectQueryParameters(t *testing.T) {
	tbl := setupTestTable()
	cfg := config.DefaultConfig().Introspect
	cfg.Public = true
	interceptor := introspect.NewInterceptor(cfg, "bld-test-query")

	// 1. ?nxp (default json)
	reqDefault := httptest.NewRequest("GET", "/auth?nxp", nil)
	recDefault := httptest.NewRecorder()
	interceptor.Intercept(recDefault, reqDefault, tbl)

	if !strings.Contains(recDefault.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected ?nxp to default to application/json, got %s", recDefault.Header().Get("Content-Type"))
	}
	var defaultObj map[string]any
	if err := json.Unmarshal(recDefault.Body.Bytes(), &defaultObj); err != nil {
		t.Errorf("expected valid JSON for ?nxp default, got error: %v", err)
	}

	// 2. ?nxp=json
	reqJSON := httptest.NewRequest("GET", "/auth?nxp=json", nil)
	reqJSON.Header.Set("Accept", "text/html") // Accept header overridden by explicit query param
	recJSON := httptest.NewRecorder()
	interceptor.Intercept(recJSON, reqJSON, tbl)

	if !strings.Contains(recJSON.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected ?nxp=json to return application/json, got %s", recJSON.Header().Get("Content-Type"))
	}
	var jsonObj map[string]any
	if err := json.Unmarshal(recJSON.Body.Bytes(), &jsonObj); err != nil {
		t.Errorf("expected valid JSON for ?nxp=json, got error: %v", err)
	}

	// 3. ?nxp=html
	reqHTML := httptest.NewRequest("GET", "/auth?nxp=html", nil)
	reqHTML.Header.Set("Accept", "application/json") // Accept header overridden by explicit query param
	recHTML := httptest.NewRecorder()
	interceptor.Intercept(recHTML, reqHTML, tbl)

	if !strings.Contains(recHTML.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected ?nxp=html to return text/html, got %s", recHTML.Header().Get("Content-Type"))
	}
	htmlBody := recHTML.Body.String()
	if !strings.Contains(htmlBody, "<!DOCTYPE html>") || !strings.Contains(htmlBody, "Interactive Tester") {
		t.Errorf("expected rich HTML tester page for ?nxp=html, got: %s", htmlBody[:min(200, len(htmlBody))])
	}
}

func TestIntrospectInspect(t *testing.T) {
	tbl := setupTestTable()
	cfg := config.DefaultConfig().Introspect
	cfg.Public = true
	interceptor := introspect.NewInterceptor(cfg, "bld-test-inspect")

	// Create a dummy temp file to inspect
	tmpFile, err := os.CreateTemp("", "inspect-test-endpoint-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString("0123456789NXP_INSPECT_DATA_PAYLOAD")
	tmpFile.Close()

	// 1. Inspect JSON: GET /__nxp/inspect?url=...&from=10&length=11
	reqURL := fmt.Sprintf("/__nxp/inspect?url=%s&from=10&length=11", tmpFile.Name())
	reqJSON := httptest.NewRequest("GET", reqURL, nil)
	recJSON := httptest.NewRecorder()
	handled := interceptor.Intercept(recJSON, reqJSON, tbl)
	if !handled {
		t.Fatalf("expected /__nxp/inspect to be intercepted")
	}

	var jsonResult map[string]any
	if err := json.Unmarshal(recJSON.Body.Bytes(), &jsonResult); err != nil {
		t.Fatalf("unmarshal inspect json failed: %v", err)
	}
	if jsonResult["data_text"] != "NXP_INSPECT" {
		t.Errorf("expected data_text NXP_INSPECT, got %v", jsonResult["data_text"])
	}

	// 2. Inspect Raw bytes: GET /__nxp/inspect?url=...&from=10&length=11&format=raw
	reqRawURL := fmt.Sprintf("/__nxp/inspect?url=%s&from=10&length=11&format=raw", tmpFile.Name())
	reqRaw := httptest.NewRequest("GET", reqRawURL, nil)
	recRaw := httptest.NewRecorder()
	handledRaw := interceptor.Intercept(recRaw, reqRaw, tbl)
	if !handledRaw {
		t.Fatalf("expected raw inspect to be intercepted")
	}

	if recRaw.Body.String() != "NXP_INSPECT" {
		t.Errorf("expected raw string NXP_INSPECT, got %q", recRaw.Body.String())
	}
	if recRaw.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("expected octet-stream header, got %s", recRaw.Header().Get("Content-Type"))
	}
}

