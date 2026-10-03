package ipc_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/ipc"
)

func TestFrameSerialization(t *testing.T) {
	body := []byte("hello world payload")
	frame := ipc.NewFrame(ipc.FrameTypeRequest, "req-1", body)
	frame.Header.Metadata["source"] = "test"

	var buf bytes.Buffer
	if err := ipc.WriteFrame(&buf, frame); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	read, err := ipc.ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if read.Header.Type != ipc.FrameTypeRequest {
		t.Errorf("type mismatch: got %s, want %s", read.Header.Type, ipc.FrameTypeRequest)
	}
	if read.Header.ID != "req-1" {
		t.Errorf("id mismatch: got %s, want req-1", read.Header.ID)
	}
	if read.Header.Metadata["source"] != "test" {
		t.Errorf("metadata mismatch: got %v", read.Header.Metadata)
	}
	if string(read.Body) != string(body) {
		t.Errorf("body mismatch: got %s, want %s", string(read.Body), string(body))
	}
}

func TestHelloAndReadyFrames(t *testing.T) {
	hello := &abi.Hello{
		ABIVersion: abi.ABIVersion,
		WorkerName: "worker-demo",
		BuildID:    "bld-xyz",
		PID:        1234,
		Routes: []abi.Route{
			{
				ID:        "auth.index",
				Transport: abi.TransportREST,
				Method:    "GET",
				Path:      "/auth",
			},
		},
	}

	var buf bytes.Buffer
	if err := ipc.SendHello(&buf, hello); err != nil {
		t.Fatalf("SendHello failed: %v", err)
	}

	readHello, err := ipc.ReadHello(&buf)
	if err != nil {
		t.Fatalf("ReadHello failed: %v", err)
	}

	if readHello.WorkerName != "worker-demo" {
		t.Errorf("expected worker-demo, got %s", readHello.WorkerName)
	}
	if len(readHello.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(readHello.Routes))
	}

	// Ready frame
	ready := &abi.Ready{
		ABIVersion: abi.ABIVersion,
		Status:     "ready",
		Message:    "route swap complete",
	}

	buf.Reset()
	if err := ipc.SendReady(&buf, ready); err != nil {
		t.Fatalf("SendReady failed: %v", err)
	}

	readReady, err := ipc.ReadReady(&buf)
	if err != nil {
		t.Fatalf("ReadReady failed: %v", err)
	}

	if readReady.Status != "ready" || readReady.Message != "route swap complete" {
		t.Errorf("unexpected ready frame: %+v", readReady)
	}
}

func TestClientServerRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ipc-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "test.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer listener.Close()

	srv := ipc.NewServer()
	srv.Handle(ipc.FrameTypeRequest, func(f *ipc.Frame) (*ipc.Frame, error) {
		echoBody := append([]byte("echo: "), f.Body...)
		resp := ipc.NewFrame(ipc.FrameTypeResponse, f.Header.ID, echoBody)
		return resp, nil
	})

	go func() {
		_ = srv.Serve(listener)
	}()
	defer srv.Shutdown(context.Background())

	client, err := ipc.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ipc.NewFrame(ipc.FrameTypeRequest, "req-42", []byte("ping"))
	resp, err := client.RoundTrip(ctx, req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if resp.Header.ID != "req-42" {
		t.Errorf("response ID mismatch: got %s, want req-42", resp.Header.ID)
	}
	if string(resp.Body) != "echo: ping" {
		t.Errorf("response body mismatch: got %s, want 'echo: ping'", string(resp.Body))
	}
}
