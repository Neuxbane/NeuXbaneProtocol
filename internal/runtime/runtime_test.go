package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/runtime"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/worker"
)

// Helper process used to test real worker subprocess lifecycle.
func TestHelperWorkerProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_WORKER") != "1" {
		return
	}

	workerName := os.Getenv("WORKER_NAME")
	buildID := os.Getenv("BUILD_ID")
	sockPath := os.Getenv("WORKER_SOCK")

	w := worker.NewRuntime(workerName, buildID, sockPath)
	w.RegisterHandler(
		abi.Route{
			ID:        "test.ping",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/ping",
		},
		func(req *abi.Request) (*abi.Response, error) {
			return abi.NewJSONResponse(200, map[string]string{
				"message": "pong",
				"worker":  workerName,
				"build":   buildID,
			})
		},
	)

	if err := w.Start(context.Background()); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestRegistry(t *testing.T) {
	reg := runtime.NewRegistry()

	inst := &runtime.WorkerInstance{
		Name:    "worker-auth",
		BuildID: "bld-1",
		PID:     100,
		Routes: []abi.Route{
			{
				ID:        "auth.login",
				Transport: abi.TransportREST,
				Method:    "POST",
				Path:      "/auth/login",
			},
		},
	}

	reg.Register(inst)

	// Lookup by name
	found, ok := reg.Get("worker-auth")
	if !ok || found.BuildID != "bld-1" {
		t.Fatalf("worker lookup by name failed")
	}

	// Lookup by route handler ID
	foundByHandler, ok := reg.FindByHandler("auth.login")
	if !ok || foundByHandler.Name != "worker-auth" {
		t.Fatalf("worker lookup by handler ID failed")
	}

	if len(reg.All()) != 1 {
		t.Errorf("expected 1 worker, got %d", len(reg.All()))
	}

	// Unregister
	unregistered := reg.Unregister("worker-auth")
	if unregistered == nil || unregistered.Name != "worker-auth" {
		t.Fatalf("unregister failed")
	}

	_, ok = reg.Get("worker-auth")
	if ok {
		t.Errorf("expected worker to be unregistered")
	}
}

func TestMotherEndToEndWithWorkerProcess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mother-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.DefaultConfig()
	cfg.SocketDir = filepath.Join(tmpDir, "socks")
	cfg.DefineDir = filepath.Join(tmpDir, "define")

	mother, err := runtime.NewMother(cfg)
	if err != nil {
		t.Fatalf("NewMother failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mother.Start(ctx); err != nil {
		t.Fatalf("mother.Start failed: %v", err)
	}
	defer mother.Shutdown(context.Background())

	testBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable failed: %v", err)
	}

	// Create wrapper script setting GO_WANT_HELPER_WORKER=1
	wrapperScript := filepath.Join(tmpDir, "run-worker.sh")
	scriptContent := "#!/bin/sh\nexport GO_WANT_HELPER_WORKER=1\nexec " + testBin + " -test.run=TestHelperWorkerProcess\n"
	if err := os.WriteFile(wrapperScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write wrapper script failed: %v", err)
	}

	// 1. Spawn Worker v1
	inst1, err := mother.Supervisor().SpawnWorker(ctx, "worker-test", "v1", wrapperScript)
	if err != nil {
		t.Fatalf("spawn worker v1 failed: %v", err)
	}
	if inst1.BuildID != "v1" {
		t.Errorf("expected buildID v1, got %s", inst1.BuildID)
	}

	// Verify route is present in mother table
	route, ok := mother.Table().Self("/ping")
	if !ok || route.ID != "test.ping" {
		t.Fatalf("expected /ping route in mother table")
	}

	// 2. Dispatch request to worker v1
	req1 := abi.NewRequest(abi.TransportREST, "GET", "/ping")
	resp1, err := mother.Dispatch(ctx, req1)
	if err != nil {
		t.Fatalf("dispatch req1 failed: %v", err)
	}
	if resp1.Status != 200 {
		t.Errorf("expected status 200, got %d", resp1.Status)
	}

	// 3. Hot swap to Worker v2
	inst2, err := mother.Supervisor().SpawnWorker(ctx, "worker-test", "v2", wrapperScript)
	if err != nil {
		t.Fatalf("spawn worker v2 failed: %v", err)
	}
	if inst2.BuildID != "v2" {
		t.Errorf("expected buildID v2, got %s", inst2.BuildID)
	}

	// 4. Dispatch request to worker v2 (verify atomic swap without dropped request)
	req2 := abi.NewRequest(abi.TransportREST, "GET", "/ping")
	resp2, err := mother.Dispatch(ctx, req2)
	if err != nil {
		t.Fatalf("dispatch req2 failed: %v", err)
	}
	if resp2.Status != 200 {
		t.Errorf("expected status 200, got %d", resp2.Status)
	}

	// Verify old worker v1 is replaced in registry
	activeInst, ok := mother.Registry().Get("worker-test")
	if !ok || activeInst.BuildID != "v2" {
		t.Errorf("expected active worker to be v2")
	}
}
