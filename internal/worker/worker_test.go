package worker_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/ipc"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/worker"
)

func TestWorkerLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "mother.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer listener.Close()

	helloReceived := make(chan *abi.Hello, 1)

	// Mock mother supervisor
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// 1. Read Hello
		hello, err := ipc.ReadHello(conn)
		if err != nil {
			t.Errorf("mother read hello failed: %v", err)
			return
		}
		helloReceived <- hello

		// 2. Send Ready
		ready := &abi.Ready{ABIVersion: abi.ABIVersion, Status: "ready"}
		if err := ipc.SendReady(conn, ready); err != nil {
			t.Errorf("mother send ready failed: %v", err)
			return
		}

		// 3. Send test request
		req := &abi.Request{
			ID:        "req-1",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/hello",
		}
		reqBody, _ := json.Marshal(req)
		reqFrame := ipc.NewFrame(ipc.FrameTypeRequest, "req-1", reqBody)
		reqFrame.Header.Metadata["handler_id"] = "test.hello"

		if err := ipc.WriteFrame(conn, reqFrame); err != nil {
			t.Errorf("mother write request frame failed: %v", err)
			return
		}

		// 4. Read response
		respFrame, err := ipc.ReadFrame(conn)
		if err != nil {
			t.Errorf("mother read response frame failed: %v", err)
			return
		}

		var resp abi.Response
		if err := json.Unmarshal(respFrame.Body, &resp); err != nil {
			t.Errorf("mother unmarshal response failed: %v", err)
			return
		}

		if resp.Status != 200 || string(resp.Body) != `"world"` {
			t.Errorf("unexpected response: %+v, body: %s", resp, string(resp.Body))
		}

		// 5. Send Drain
		drainFrame := ipc.NewFrame(ipc.FrameTypeDrain, "drain", nil)
		_ = ipc.WriteFrame(conn, drainFrame)
	}()

	// Initialize worker
	w := worker.NewRuntime("test-worker", "bld-1", sockPath)
	w.RegisterHandler(
		abi.Route{
			ID:        "test.hello",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/hello",
		},
		func(req *abi.Request) (*abi.Response, error) {
			return abi.NewJSONResponse(200, "world")
		},
	)

	// Start worker in background
	workerErr := make(chan error, 1)
	go func() {
		workerErr <- w.Start(context.Background())
	}()

	select {
	case hello := <-helloReceived:
		if hello.WorkerName != "test-worker" {
			t.Errorf("expected worker name test-worker, got %s", hello.WorkerName)
		}
		if len(hello.Routes) != 1 {
			t.Errorf("expected 1 route in hello, got %d", len(hello.Routes))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker hello frame")
	}

	select {
	case err := <-workerErr:
		if err != nil {
			t.Errorf("worker exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker to drain and exit")
	}
}
