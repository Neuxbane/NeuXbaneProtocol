package introspect_test

import (
	"encoding/json"
	"net/http/httptest"
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
		},
	})
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.profile.patch",
			Transport: abi.TransportREST,
			Method:    "PATCH",
			Path:      "/auth/profile",
			Auth:      "required",
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
