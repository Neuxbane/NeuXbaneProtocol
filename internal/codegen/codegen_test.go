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
		{"define/index.go", "/"},
		{"define/auth/index.go", "/auth"},
		{"define/auth/login.go", "/auth/login"},
		{"define/users/[id].go", "/users/{id}"},
		{"define/users/[id]/tokens/index.go", "/users/{id}/tokens"},
		{"define/get_profile.go", "/profile"},
		{"define/auth/post_login.go", "/auth/login"},
		{"define/del_item.go", "/item"},
		{"define/ws_chat.go", "/chat"},
		{"define/mqtt_sensors.go", "/sensors"},
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
		filename  string
		wantTr    abi.Transport
		wantMeth  string
	}{
		{"get_user.go", abi.TransportREST, "GET"},
		{"post_login.go", abi.TransportREST, "POST"},
		{"put_profile.go", abi.TransportREST, "PUT"},
		{"del_order.go", abi.TransportREST, "DELETE"},
		{"patch_settings.go", abi.TransportREST, "PATCH"},
		{"ws_stream.go", abi.TransportWebSocket, "CONNECT"},
		{"grpc_service.go", abi.TransportGRPC, "POST"},
		{"udp_telemetry.go", abi.TransportUDP, "SEND"},
		{"mqtt_events.go", abi.TransportMQTT, "SUB"},
		{"nats_jobs.go", abi.TransportNATS, "SUB"},
		{"kafka_orders.go", abi.TransportKafka, "CONSUME"},
		{"index.go", abi.TransportREST, "GET"},
	}

	for _, tt := range tests {
		tr, meth := codegen.InferTransportAndMethod(tt.filename)
		if tr != tt.wantTr || meth != tt.wantMeth {
			t.Errorf("Infer(%q) = (%s, %s), want (%s, %s)", tt.filename, tr, meth, tt.wantTr, tt.wantMeth)
		}
	}
}

func TestParseDirectives(t *testing.T) {
	source := `// Package auth handles authentication.
// @route /api/v2/login
// @transport rest
// @method POST
// @auth required
// @scope admin users:write
// @ratelimit 50 100
// @topic sensors/+/temp
// @qos 1
// @retain
package auth
`

	d := codegen.ParseDirectives(source)
	if d.Route != "/api/v2/login" {
		t.Errorf("Route = %q, want /api/v2/login", d.Route)
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

func TestGenerateProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "codegen-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	defineDir := filepath.Join(tmpDir, "define")
	authDir := filepath.Join(defineDir, "auth")
	_ = os.MkdirAll(authDir, 0755)

	indexFile := filepath.Join(defineDir, "index.go")
	indexCode := `package root

import "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"

func Handler(ctx *rest.Ctx) (string, error) {
	return "welcome", nil
}
`
	_ = os.WriteFile(indexFile, []byte(indexCode), 0644)

	loginFile := filepath.Join(authDir, "post_login.go")
	loginCode := `// @auth public
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

	savedBuildID, err := codegen.ReadBuildIDFile(buildIDFile)
	if err != nil || savedBuildID != buildID {
		t.Errorf("saved build ID %q doesn't match %q", savedBuildID, buildID)
	}
}
