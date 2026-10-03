package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/introspect"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport/rest"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

func TestIntrospectEndToEnd(t *testing.T) {
	table := router.NewTable()
	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "get.index",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/",
			Shape: abi.RequestResponseShape{
				Responses: map[int]*abi.Schema{200: {Type: "object"}},
			},
		},
		WorkerName: "worker-1",
	})

	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "get.auth",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth",
			Shape: abi.RequestResponseShape{
				Responses: map[int]*abi.Schema{200: {Type: "object"}},
			},
		},
		WorkerName: "worker-1",
	})

	table.Store(&router.Entry{
		Route: abi.Route{
			ID:        "post.auth.login",
			Transport: abi.TransportREST,
			Method:    "POST",
			Path:      "/auth/login",
			Shape: abi.RequestResponseShape{
				Responses: map[int]*abi.Schema{200: {Type: "object"}},
			},
		},
		WorkerName: "worker-1",
	})

	var workerCalled atomic.Bool
	adapter := rest.NewAdapter(func(ctx context.Context, req *abi.Request) (*abi.Response, error) {
		workerCalled.Store(true)
		return abi.NewJSONResponse(200, map[string]string{"msg": "worker executed"})
	})

	cfg := config.DefaultConfig().Introspect
	cfg.Enabled = true
	cfg.Public = true
	cfg.PublicSchema = true
	cfg.HTML = true

	interceptor := introspect.NewInterceptor(cfg, "test-build-123")

	adapter.SetIntrospector(interceptor.Intercept)

	// Create test HTTP handler via adapter
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("nxp") {
			if handled := interceptor.Intercept(w, r, table); handled {
				return
			}
		}
		abiReq, _ := adapter.Normalize(r)
		_ = abiReq
	}))
	defer ts.Close()

	// 1. Test JSON introspection
	t.Run("Accept application/json", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/auth?nxp", nil)
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}

		if body["path"] != "/auth" {
			t.Errorf("expected path /auth, got %v", body["path"])
		}
		if body["self"] == nil {
			t.Errorf("expected self entry for /auth, got nil")
		}
		if workerCalled.Load() {
			t.Errorf("worker was unexpectedly called during introspection")
		}
	})

	// 2. Test text/plain ASCII tree
	t.Run("Accept text/plain", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/?nxp", nil)
		req.Header.Set("Accept", "text/plain")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/plain") {
			t.Errorf("expected Content-Type text/plain, got %q", ct)
		}
	})

	// 3. Test text/html Interactive UI
	t.Run("Accept text/html", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/auth?nxp", nil)
		req.Header.Set("Accept", "text/html")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Errorf("expected Content-Type text/html, got %q", ct)
		}
	})

	// 4. Test OpenAPI 3.1
	t.Run("Accept application/openapi+json", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/auth?nxp", nil)
		req.Header.Set("Accept", "application/openapi+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		var oapi map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&oapi); err != nil {
			t.Fatalf("failed to decode OpenAPI JSON: %v", err)
		}
		if oapi["openapi"] != "3.1.0" {
			t.Errorf("expected openapi 3.1.0, got %v", oapi["openapi"])
		}
	})

	// 5. Test AsyncAPI 3.0
	t.Run("Accept application/asyncapi+json", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/auth?nxp", nil)
		req.Header.Set("Accept", "application/asyncapi+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		var aapi map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&aapi); err != nil {
			t.Fatalf("failed to decode AsyncAPI JSON: %v", err)
		}
		if aapi["asyncapi"] != "3.0.0" {
			t.Errorf("expected asyncapi 3.0.0, got %v", aapi["asyncapi"])
		}
	})

	// 6. Test JSON Schema bundle: /__nxp/schema?nxp&build=<buildID>
	t.Run("Schema Bundle /__nxp/schema", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/__nxp/schema?nxp&build=test-build-123", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		var bundle map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
			t.Fatalf("failed to decode schema bundle JSON: %v", err)
		}
		if bundle["build_id"] != "test-build-123" {
			t.Errorf("expected build_id test-build-123, got %v", bundle["build_id"])
		}
	})
}
