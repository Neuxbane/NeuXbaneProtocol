// Package worker answers: how does a worker process bootstrap its route handlers, register with the mother supervisor, and execute incoming requests?
//
// Design Decision:
// One generic nxp-worker binary is compiled per project build, linking the generated route table
// from internal/generated. This avoids recompiling the worker runtime infrastructure on every hot reload,
// providing fast reload cycles (<500ms) while keeping workers strictly isolated in their own OS processes.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/ipc"
)

// HandlerFunc is the internal dispatch adapter for a business logic route handler.
type HandlerFunc func(*abi.Request) (*abi.Response, error)

// Runtime orchestrates the worker-side process lifecycle.
type Runtime struct {
	workerName string
	buildID    string
	sockPath   string
	routes     []abi.Route
	handlers   map[abi.HandlerID]HandlerFunc
	handlersMu sync.RWMutex

	client     *ipc.Client
	conn       net.Conn
	inFlight   atomic.Int64
	draining   atomic.Bool
	stopChan   chan struct{}
}

// NewRuntime initializes a worker Runtime instance.
func NewRuntime(workerName, buildID, sockPath string) *Runtime {
	return &Runtime{
		workerName: workerName,
		buildID:    buildID,
		sockPath:   sockPath,
		handlers:   make(map[abi.HandlerID]HandlerFunc),
		stopChan:   make(chan struct{}),
	}
}

// RegisterHandler registers a route and its handler adapter in the worker runtime.
func (r *Runtime) RegisterHandler(route abi.Route, fn HandlerFunc) {
	r.handlersMu.Lock()
	defer r.handlersMu.Unlock()
	r.routes = append(r.routes, route)
	r.handlers[route.ID] = fn
}

// Routes returns the current routes registered in this worker.
func (r *Runtime) Routes() []abi.Route {
	r.handlersMu.RLock()
	defer r.handlersMu.RUnlock()
	cp := make([]abi.Route, len(r.routes))
	copy(cp, r.routes)
	return cp
}

// Start connects to WORKER_SOCK, transmits Hello, awaits Ready, and enters request serving.
func (r *Runtime) Start(ctx context.Context) error {
	sock := r.sockPath
	if sock == "" {
		sock = os.Getenv("WORKER_SOCK")
	}
	if sock == "" {
		return fmt.Errorf("WORKER_SOCK environment variable or sockPath not specified")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return fmt.Errorf("connect to mother socket %s: %w", sock, err)
	}
	r.conn = conn

	// 1. Send Hello frame
	hello := &abi.Hello{
		ABIVersion: abi.ABIVersion,
		WorkerName: r.workerName,
		BuildID:    r.buildID,
		PID:        os.Getpid(),
		Routes:     r.Routes(),
	}

	if err := ipc.SendHello(conn, hello); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send hello frame: %w", err)
	}

	// 2. Wait for Ready frame
	ready, err := ipc.ReadReady(conn)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("await ready frame: %w", err)
	}
	if ready.Status != "ready" {
		_ = conn.Close()
		return fmt.Errorf("mother rejected worker readiness: %s", ready.Message)
	}

	// 3. Listen for OS termination signals (SIGTERM, SIGINT)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-sigChan:
			_ = r.Drain(context.Background(), 5*time.Second)
		case <-r.stopChan:
		}
	}()

	// 4. Enter serve loop
	return r.serveLoop()
}

func (r *Runtime) serveLoop() error {
	defer func() {
		if r.conn != nil {
			_ = r.conn.Close()
		}
	}()

	var writeMu sync.Mutex

	for {
		frame, err := ipc.ReadFrame(r.conn)
		if err != nil {
			if r.draining.Load() || err == io.EOF {
				return nil
			}
			return fmt.Errorf("worker read frame error: %w", err)
		}

		switch frame.Header.Type {
		case ipc.FrameTypeDrain:
			return r.Drain(context.Background(), 5*time.Second)
		case ipc.FrameTypePing:
			pong := ipc.NewFrame(ipc.FrameTypePong, frame.Header.ID, nil)
			writeMu.Lock()
			_ = ipc.WriteFrame(r.conn, pong)
			writeMu.Unlock()
		case ipc.FrameTypeRequest:
			r.inFlight.Add(1)
			go func(f *ipc.Frame) {
				defer r.inFlight.Add(-1)
				respFrame := r.dispatch(f)
				writeMu.Lock()
				_ = ipc.WriteFrame(r.conn, respFrame)
				writeMu.Unlock()
			}(frame)
		}
	}
}

func (r *Runtime) dispatch(f *ipc.Frame) *ipc.Frame {
	var req abi.Request
	if err := json.Unmarshal(f.Body, &req); err != nil {
		errResp := abi.NewErrorResponse(errors.New(errors.CodeBadRequest, "malformed request frame", 400))
		body, _ := json.Marshal(errResp)
		return ipc.NewFrame(ipc.FrameTypeResponse, f.Header.ID, body)
	}

	handlerID := abi.HandlerID(f.Header.Metadata["handler_id"])
	if handlerID == "" {
		handlerID = abi.HandlerID(req.RoutePath)
	}

	r.handlersMu.RLock()
	handler, ok := r.handlers[handlerID]
	r.handlersMu.RUnlock()

	if !ok {
		errResp := abi.NewErrorResponse(errors.New(errors.CodeNotFound, fmt.Sprintf("handler %q not registered in worker", handlerID), 404))
		body, _ := json.Marshal(errResp)
		return ipc.NewFrame(ipc.FrameTypeResponse, f.Header.ID, body)
	}

	resp, err := handler(&req)
	if err != nil {
		resp = abi.NewErrorResponse(err)
	} else if resp == nil {
		resp = abi.NewResponse(204, nil)
	}

	respBytes, _ := json.Marshal(resp)
	respFrame := ipc.NewFrame(ipc.FrameTypeResponse, f.Header.ID, respBytes)
	return respFrame
}

// Drain gracefully drains in-flight requests and exits.
func (r *Runtime) Drain(ctx context.Context, timeout time.Duration) error {
	if !r.draining.CompareAndSwap(false, true) {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for r.inFlight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	close(r.stopChan)
	if r.conn != nil {
		_ = r.conn.Close()
	}
	return nil
}
