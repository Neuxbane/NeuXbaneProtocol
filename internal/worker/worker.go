// Package worker answers: how does a worker process bootstrap its route handlers, register with the mother supervisor, and execute incoming requests?
//
// Design Decision:
// One generic nxp-worker binary is compiled per project build, linking the generated route table
// from the generated package. This avoids recompiling the worker runtime infrastructure on every hot reload,
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

	"github.com/Neuxbane/NeuXbaneProtocol/internal/ipc"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
)

// HandlerFunc is the internal dispatch adapter for a business logic route handler.
type HandlerFunc func(*abi.Request) (*abi.Response, error)

// StreamHandlerFunc is the internal dispatch adapter for a business logic streaming route handler.
type StreamHandlerFunc func(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error

// Runtime orchestrates the worker-side process lifecycle.
type Runtime struct {
	workerName     string
	buildID        string
	sockPath       string
	routes         []abi.Route
	handlers       map[abi.HandlerID]HandlerFunc
	streamHandlers map[abi.HandlerID]StreamHandlerFunc
	handlersMu     sync.RWMutex

	activeStreams map[string]context.CancelFunc
	activeInChans map[string]chan []byte
	streamsMu     sync.Mutex

	conn     net.Conn
	inFlight atomic.Int64
	draining atomic.Bool
	stopChan chan struct{}
}

// NewRuntime initializes a worker Runtime instance.
func NewRuntime(workerName, buildID, sockPath string) *Runtime {
	return &Runtime{
		workerName:     workerName,
		buildID:        buildID,
		sockPath:       sockPath,
		handlers:       make(map[abi.HandlerID]HandlerFunc),
		streamHandlers: make(map[abi.HandlerID]StreamHandlerFunc),
		activeStreams:  make(map[string]context.CancelFunc),
		activeInChans:  make(map[string]chan []byte),
		stopChan:       make(chan struct{}),
	}
}

// RegisterHandler registers a route and its handler adapter in the worker runtime.
func (r *Runtime) RegisterHandler(route abi.Route, fn HandlerFunc) {
	r.handlersMu.Lock()
	defer r.handlersMu.Unlock()
	r.routes = append(r.routes, route)
	r.handlers[route.ID] = fn
}

// RegisterStreamHandler registers a streaming route and its handler adapter in the worker runtime.
func (r *Runtime) RegisterStreamHandler(route abi.Route, fn StreamHandlerFunc) {
	r.handlersMu.Lock()
	defer r.handlersMu.Unlock()
	r.routes = append(r.routes, route)
	r.streamHandlers[route.ID] = fn
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
		case ipc.FrameTypeStreamStart:
			r.inFlight.Add(1)
			go func(f *ipc.Frame) {
				defer r.inFlight.Add(-1)
				r.handleStreamStart(f, &writeMu)
			}(frame)
		case ipc.FrameTypeStreamData:
			r.handleStreamData(frame)
		case ipc.FrameTypeStreamCancel:
			r.handleStreamCancel(frame)
		}
	}
}

func (r *Runtime) handleStreamStart(f *ipc.Frame, writeMu *sync.Mutex) {
	var req abi.Request
	if err := json.Unmarshal(f.Body, &req); err != nil {
		endFrame := ipc.NewFrame(ipc.FrameTypeStreamEnd, f.Header.ID, nil)
		writeMu.Lock()
		_ = ipc.WriteFrame(r.conn, endFrame)
		writeMu.Unlock()
		return
	}

	handlerID := abi.HandlerID(f.Header.Metadata["handler_id"])
	if handlerID == "" {
		handlerID = abi.HandlerID(req.RoutePath)
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	inCh := make(chan []byte, 64)

	r.streamsMu.Lock()
	r.activeStreams[f.Header.ID] = cancel
	r.activeInChans[f.Header.ID] = inCh
	r.streamsMu.Unlock()

	defer func() {
		r.streamsMu.Lock()
		delete(r.activeStreams, f.Header.ID)
		delete(r.activeInChans, f.Header.ID)
		r.streamsMu.Unlock()

		endFrame := ipc.NewFrame(ipc.FrameTypeStreamEnd, f.Header.ID, nil)
		writeMu.Lock()
		_ = ipc.WriteFrame(r.conn, endFrame)
		writeMu.Unlock()
	}()

	sink := &workerStreamSink{
		id:      f.Header.ID,
		conn:    r.conn,
		writeMu: writeMu,
		ctx:     streamCtx,
	}

	r.handlersMu.RLock()
	sHandler, hasStream := r.streamHandlers[handlerID]
	handler, hasReq := r.handlers[handlerID]
	r.handlersMu.RUnlock()

	if hasStream {
		_ = sHandler(streamCtx, &req, sink, inCh)
	} else if hasReq {
		resp, err := handler(&req)
		if err == nil && resp != nil && len(resp.Body) > 0 {
			_ = sink.Send(resp.Body)
		}
	}
}

func (r *Runtime) handleStreamData(f *ipc.Frame) {
	r.streamsMu.Lock()
	inCh, ok := r.activeInChans[f.Header.ID]
	r.streamsMu.Unlock()

	if ok && inCh != nil {
		select {
		case inCh <- f.Body:
		default:
		}
	}
}

func (r *Runtime) handleStreamCancel(f *ipc.Frame) {
	r.streamsMu.Lock()
	cancel, ok := r.activeStreams[f.Header.ID]
	r.streamsMu.Unlock()

	if ok && cancel != nil {
		cancel()
	}
}

type workerStreamSink struct {
	id      string
	conn    net.Conn
	writeMu *sync.Mutex
	ctx     context.Context
}

func (s *workerStreamSink) Send(data any) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}

	var b []byte
	switch v := data.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}

	frame := ipc.NewFrame(ipc.FrameTypeStreamData, s.id, b)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return ipc.WriteFrame(s.conn, frame)
}

func (s *workerStreamSink) Close() error {
	return nil
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

	r.streamsMu.Lock()
	for _, cancel := range r.activeStreams {
		if cancel != nil {
			cancel()
		}
	}
	r.streamsMu.Unlock()

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
