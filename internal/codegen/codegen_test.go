package codegen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/codegen"
)

func TestComputeRoutePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"define/get/handler.go", "/"},
		{"define/auth/get/handler.go", "/auth"},
		{"define/auth/login.go", "/auth/login"},
		{"define/users/[id].go", "/users/{id}"},
		{"define/users/[id]/tokens/get/handler.go", "/users/{id}/tokens"},
		{"define/get_profile.go", "/profile"},
		{"define/auth/post_login.go", "/auth/login"},
		{"define/del_item.go", "/item"},
		{"define/ws_chat.go", "/chat"},
		{"define/mqtt_sensors.go", "/sensors"},
		// Folder-routing convention: method folder + handler.go, or bare method file.
		{"define/agents/get/handler.go", "/agents"},
		{"define/agents/post/handler.go", "/agents"},
		{"define/agents/@id/get/handler.go", "/agents/{id}"},
		{"define/agents/@id/patch/handler.go", "/agents/{id}"},
		{"define/agents/@id/delete/handler.go", "/agents/{id}"},
		{"define/agents/@id/get.go", "/agents/{id}"},
		{"define/chat/room/ws/handler.go", "/chat/room"},
		{"define/sensors/telemetry/mqtt/handler.go", "/sensors/telemetry"},
		{"define/storage/download/get/handler.go", "/storage/download"},
	}

	for _, tt := range tests {
		got := codegen.ComputeRoutePath(tt.input)
		if got != tt.expected {
			t.Errorf("ComputeRoutePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestInferTransportAndMethod(t *testing.T) {
	tests := []struct {
		path     string
		wantTr   abi.Transport
		wantMeth string
	}{
		// Method folder + handler.go
		{"agents/get/handler.go", abi.TransportREST, "GET"},
		{"agents/post/handler.go", abi.TransportREST, "POST"},
		{"agents/@id/patch/handler.go", abi.TransportREST, "PATCH"},
		{"agents/@id/delete/handler.go", abi.TransportREST, "DELETE"},
		{"chat/room/ws/handler.go", abi.TransportWebSocket, "CONNECT"},
		{"sensors/telemetry/mqtt/handler.go", abi.TransportMQTT, "SUB"},
		// Bare method filename
		{"agents/@id/get.go", abi.TransportREST, "GET"},
		{"agents/@id/patch.go", abi.TransportREST, "PATCH"},
		{"agents/@id/delete.go", abi.TransportREST, "DELETE"},
		// Legacy prefix form
		{"get_user.go", abi.TransportREST, "GET"},
		{"post_login.go", abi.TransportREST, "POST"},
		{"ws_stream.go", abi.TransportWebSocket, "CONNECT"},
		{"grpc_service.go", abi.TransportGRPC, "POST"},
		{"udp_telemetry.go", abi.TransportUDP, "SEND"},
		{"mqtt_events.go", abi.TransportMQTT, "SUB"},
		{"nats_jobs.go", abi.TransportNATS, "SUB"},
		{"kafka_orders.go", abi.TransportKafka, "CONSUME"},
		{"index.go", abi.TransportREST, "GET"},
	}

	for _, tt := range tests {
		tr, meth := codegen.InferTransportAndMethod(tt.path)
		if tr != tt.wantTr || meth != tt.wantMeth {
			t.Errorf("Infer(%q) = (%s, %s), want (%s, %s)", tt.path, tr, meth, tt.wantTr, tt.wantMeth)
		}
	}
}

func TestParseDirectives(t *testing.T) {
	source := `// Package auth handles authentication.
// @transport rest
// @method POST
// @auth required
// @scope admin users:write
// @guard auth, user/admin
// @ratelimit 50 100
// @topic sensors/+/temp
// @qos 1
// @retain
// @desc Authenticate a user and issue a session token.
package auth
`

	d := codegen.ParseDirectives(source)
	if d.Description != "Authenticate a user and issue a session token." {
		t.Errorf("Description = %q, want %q", d.Description, "Authenticate a user and issue a session token.")
	}
	if d.Transport != abi.TransportREST {
		t.Errorf("Transport = %s, want rest", d.Transport)
	}
	if d.Method != "POST" {
		t.Errorf("Method = %s, want POST", d.Method)
	}
	if d.Auth != "required" {
		t.Errorf("Auth = %s, want required", d.Auth)
	}
	if len(d.Scopes) != 2 || d.Scopes[0] != "admin" || d.Scopes[1] != "users:write" {
		t.Errorf("Scopes = %v, want [admin, users:write]", d.Scopes)
	}
	if len(d.Guards) != 2 || d.Guards[0] != "auth" || d.Guards[1] != "user/admin" {
		t.Errorf("Guards = %v, want [auth, user/admin]", d.Guards)
	}
	if d.RateLimit == nil || d.RateLimit.RPS != 50 || d.RateLimit.Burst != 100 {
		t.Errorf("RateLimit = %+v, want {50, 100}", d.RateLimit)
	}
	if d.Topic != "sensors/+/temp" || d.QoS != 1 || !d.Retain {
		t.Errorf("PubSub directives mismatch: topic=%s qos=%d retain=%t", d.Topic, d.QoS, d.Retain)
	}
}

func TestInspectHandlerSource(t *testing.T) {
	src := `package profile

import "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"

type Result struct {
	Name string
}

func Handler(ctx *rest.Ctx) (Result, error) {
	return Result{Name: "test"}, nil
}
`

	info, err := codegen.InspectHandlerSource("profile.go", src)
	if err != nil {
		t.Fatalf("InspectHandlerSource failed: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil HandlerInfo")
	}
	if info.FunctionName != "Handler" {
		t.Errorf("FunctionName = %s, want Handler", info.FunctionName)
	}
	if info.Domain != "rest" {
		t.Errorf("Domain = %s, want rest", info.Domain)
	}
	if info.ResultType != "Result" {
		t.Errorf("ResultType = %s, want Result", info.ResultType)
	}
}

func TestInspectHandlerNestedStruct(t *testing.T) {
	src := `package engines

import "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"

type ModelInput struct {
	ModelID string ` + "`json:\"model_id\" validate:\"required\"`" + `
	Name    string ` + "`json:\"name\"`" + `
}

type Request struct {
	Name   string       ` + "`json:\"name\" validate:\"required\"`" + `
	Models []ModelInput ` + "`json:\"models\"`" + `
}

func Handler(ctx *rest.Ctx) (any, error) {
	return nil, nil
}
`

	info, err := codegen.InspectHandlerSource("engines.go", src)
	if err != nil {
		t.Fatalf("InspectHandlerSource failed: %v", err)
	}
	if info == nil || info.InputSchema == nil {
		t.Fatal("expected non-nil input schema")
	}
	// The heuristic must pick Request, not ModelInput.
	if _, ok := info.InputSchema.Properties["models"]; !ok {
		t.Fatalf("expected Request schema with 'models' property, got %+v", info.InputSchema.Properties)
	}
	if _, ok := info.InputSchema.Properties["model_id"]; ok {
		t.Fatal("input schema was flattened to ModelInput instead of Request")
	}
	// Nested array items must carry the real struct properties.
	items := info.InputSchema.Properties["models"].Items
	if items == nil || items.Properties == nil {
		t.Fatalf("expected resolved nested items schema, got %+v", items)
	}
	if _, ok := items.Properties["model_id"]; !ok {
		t.Errorf("nested items missing model_id property: %+v", items.Properties)
	}
	if len(items.Required) != 1 || items.Required[0] != "model_id" {
		t.Errorf("nested items required = %v, want [model_id]", items.Required)
	}
}

func TestGenerateProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "codegen-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	defineDir := filepath.Join(tmpDir, "define")
	authDir := filepath.Join(defineDir, "auth")
	_ = os.MkdirAll(authDir, 0755)

	// Root GET handler lives in a method folder: define/get/handler.go.
	rootGetDir := filepath.Join(defineDir, "get")
	_ = os.MkdirAll(rootGetDir, 0755)
	indexFile := filepath.Join(rootGetDir, "handler.go")
	indexCode := `// @guard
package get

import "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"

func Handler(ctx *rest.Ctx) (string, error) {
	return "welcome", nil
}
`
	_ = os.WriteFile(indexFile, []byte(indexCode), 0644)

	// Guard implementation: define/auth/index.go implements guard "auth".
	guardFile := filepath.Join(authDir, "index.go")
	guardCode := `package auth

import "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"

func Guard(ctx *rest.Ctx) error {
	return nil
}
`
	_ = os.WriteFile(guardFile, []byte(guardCode), 0644)

	loginFile := filepath.Join(authDir, "post_login.go")
	loginCode := `// @auth public
// @guard auth
package auth

import "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"

func Handler(ctx *rest.Ctx) (string, error) {
	return "token", nil
}
`
	_ = os.WriteFile(loginFile, []byte(loginCode), 0644)

	outGen := filepath.Join(tmpDir, "internal", "generated", "routes_gen.go")
	buildIDFile := filepath.Join(tmpDir, ".nxp-build-id")

	buildID, err := codegen.GenerateProject(defineDir, "github.com/Neuxbane/NeuXbaneProtocol", outGen, buildIDFile)
	if err != nil {
		t.Fatalf("GenerateProject failed: %v", err)
	}

	if buildID == "" {
		t.Fatal("empty buildID")
	}

	genBytes, err := os.ReadFile(outGen)
	if err != nil {
		t.Fatalf("read outGen failed: %v", err)
	}

	genStr := string(genBytes)
	t.Logf("Generated Code:\n%s", genStr)
	if !strings.Contains(genStr, "GeneratedRoutes = []abi.Route{") {
		t.Errorf("generated code missing GeneratedRoutes")
	}
	if !strings.Contains(genStr, `"post.auth.login"`) || !strings.Contains(genStr, `"/auth/login"`) {
		t.Errorf("generated code missing /auth/login route")
	}
	if !strings.Contains(genStr, `"get.index"`) {
		t.Errorf("generated code missing root route get.index")
	}
	if !strings.Contains(genStr, "GeneratedGuards = map[string]func(*abi.Request) error{") {
		t.Errorf("generated code missing GeneratedGuards")
	}
	if !strings.Contains(genStr, `"guard.auth"`) {
		t.Errorf("generated code missing guard guard.auth")
	}
	if !strings.Contains(genStr, `[]string{"guard.auth"}`) {
		t.Errorf("generated code missing route guard wiring")
	}

	savedBuildID, err := codegen.ReadBuildIDFile(buildIDFile)
	if err != nil || savedBuildID != buildID {
		t.Errorf("saved build ID %q doesn't match %q", savedBuildID, buildID)
	}
}

func TestGuardRestrictions(t *testing.T) {
	// 1. Missing @guard should fail
	t.Run("MissingGuardFails", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "guard-test-missing-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		_ = os.WriteFile(filepath.Join(tmpDir, "get.go"), []byte("package get\n"), 0644)
		_, err = codegen.WalkDefineTree(tmpDir)
		if err == nil {
			t.Fatal("expected error for missing @guard, got nil")
		}
		if !strings.Contains(err.Error(), "missing required @guard directive") {
			t.Fatalf("expected missing required @guard directive error, got: %v", err)
		}
	})

	// 2. Bare @guard for none should succeed
	t.Run("BareGuardNoneSucceeds", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "guard-test-none-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		_ = os.WriteFile(filepath.Join(tmpDir, "get.go"), []byte("// @guard\npackage get\n"), 0644)
		routes, err := codegen.WalkDefineTree(tmpDir)
		if err != nil {
			t.Fatalf("expected success for bare @guard, got: %v", err)
		}
		if len(routes) != 1 || len(routes[0].Guards) != 0 {
			t.Fatalf("expected 1 route with 0 guards, got: %+v", routes)
		}
	})

	// 3. Unknown guard should fail
	t.Run("UnknownGuardFails", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "guard-test-unknown-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		_ = os.WriteFile(filepath.Join(tmpDir, "get.go"), []byte("// @guard nonexistent\npackage get\n"), 0644)
		_, err = codegen.WalkDefineTree(tmpDir)
		if err == nil {
			t.Fatal("expected error for unknown guard, got nil")
		}
		if !strings.Contains(err.Error(), "unknown guard \"nonexistent\"") {
			t.Fatalf("expected unknown guard error, got: %v", err)
		}
	})

	// 4. Multiple guards (like @guard account/auth account/admin) should resolve
	t.Run("MultipleGuardsResolve", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "guard-test-multi-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		accountAuthDir := filepath.Join(tmpDir, "account", "auth")
		accountAdminDir := filepath.Join(tmpDir, "account", "admin")
		_ = os.MkdirAll(accountAuthDir, 0755)
		_ = os.MkdirAll(accountAdminDir, 0755)

		_ = os.WriteFile(filepath.Join(accountAuthDir, "index.go"), []byte("package auth\n"), 0644)
		_ = os.WriteFile(filepath.Join(accountAdminDir, "index.go"), []byte("package admin\n"), 0644)

		_ = os.WriteFile(filepath.Join(tmpDir, "get.go"), []byte("// @guard account/auth account/admin\npackage get\n"), 0644)
		routes, err := codegen.WalkDefineTree(tmpDir)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("expected 1 route, got %d", len(routes))
		}
		expectedGuards := []string{"guard.account.auth", "guard.account.admin"}
		if len(routes[0].Guards) != 2 || routes[0].Guards[0] != expectedGuards[0] || routes[0].Guards[1] != expectedGuards[1] {
			t.Fatalf("expected guards %v, got %v", expectedGuards, routes[0].Guards)
		}
	})
}

