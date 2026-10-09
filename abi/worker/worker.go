package worker

import (
	"context"
	"encoding/binary"
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

	"abi"
)

// HandlerFunc is the internal dispatch adapter for a business logic route handler.
type HandlerFunc func(*abi.Request) (*abi.Response, error)

// StreamHandlerFunc is the internal dispatch adapter for a business logic streaming route handler.
type StreamHandlerFunc func(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error

type frameHeader struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	BodyLength int               `json:"body_length"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type frame struct {
	Header frameHeader
	Body   []byte
}

func readFrame(r io.Reader) (*frame, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	headerLen := binary.BigEndian.Uint32(lenBuf[:])
	if headerLen == 0 || headerLen > 16*1024*1024 {
		return nil, fmt.Errorf("invalid frame header length: %d", headerLen)
	}
	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, fmt.Errorf("read frame header: %w", err)
	}
	var header frameHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshal frame header: %w", err)
	}
	var body []byte
	if header.BodyLength > 0 {
		body = make([]byte, header.BodyLength)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("read frame body: %w", err)
		}
	}
	return &frame{Header: header, Body: body}, nil
}

func writeFrame(w io.Writer, f *frame) error {
	f.Header.BodyLength = len(f.Body)
	headerBytes, err := json.Marshal(f.Header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}
	headerLen := uint32(len(headerBytes))
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], headerLen)
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(headerBytes); err != nil {
		return err
	}
	if len(f.Body) > 0 {
		if _, err := w.Write(f.Body); err != nil {
			return err
		}
	}
	return nil
}

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

// Start connects to WORKER_SOCK, transmits Hello, awaits Ready, and enters request serving.
func (r *Runtime) Start(ctx context.Context) error {
	sock := r.sockPath
	if sock == "" {
		sock = os.Getenv("WORKER_SOCK")
	}
	if sock == "" {
		return fmt.Errorf("WORKER_SOCK environment variable not specified")
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
		Routes:     r.routes,
	}
	helloBytes, err := json.Marshal(hello)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("marshal hello: %w", err)
	}

	hFrame := &frame{
		Header: frameHeader{
			Type: "hello",
			ID:   r.workerName,
			Metadata: map[string]string{
				"abi_version": abi.ABIVersion,
				"worker_name": r.workerName,
				"build_id":    r.buildID,
			},
		},
		Body: helloBytes,
	}
	if err := writeFrame(conn, hFrame); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send hello frame: %w", err)
	}

	// 2. Wait for Ready frame
	readyFrame, err := readFrame(conn)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("read ready frame: %w", err)
	}
	var ready abi.Ready
	if err := json.Unmarshal(readyFrame.Body, &ready); err != nil {
		_ = conn.Close()
		return fmt.Errorf("unmarshal ready frame: %w", err)
	}
	if ready.Status != "ready" {
		_ = conn.Close()
		return fmt.Errorf("mother rejected worker readiness: %s", ready.Message)
	}

	// 3. Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-sigChan:
			_ = r.Drain(context.Background(), 5*time.Second)
		case <-r.stopChan:
		}
	}()

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
		f, err := readFrame(r.conn)
		if err != nil {
			if r.draining.Load() || err == io.EOF {
				return nil
			}
			return fmt.Errorf("worker read frame error: %w", err)
		}

		switch f.Header.Type {
		case "drain":
			return r.Drain(context.Background(), 5*time.Second)
		case "ping":
			pong := &frame{Header: frameHeader{Type: "pong", ID: f.Header.ID}}
			writeMu.Lock()
			_ = writeFrame(r.conn, pong)
			writeMu.Unlock()
		case "request":
			r.inFlight.Add(1)
			go func(reqFrame *frame) {
				defer r.inFlight.Add(-1)
				respFrame := r.dispatch(reqFrame)
				writeMu.Lock()
				_ = writeFrame(r.conn, respFrame)
				writeMu.Unlock()
			}(f)
		case "stream_start":
			r.inFlight.Add(1)
			go func(streamFrame *frame) {
				defer r.inFlight.Add(-1)
				r.handleStreamStart(streamFrame, &writeMu)
			}(f)
		case "stream_data":
			r.handleStreamData(f)
		case "stream_cancel":
			r.handleStreamCancel(f)
		}
	}
}

func (r *Runtime) handleStreamStart(f *frame, writeMu *sync.Mutex) {
	var req abi.Request
	if err := json.Unmarshal(f.Body, &req); err != nil {
		endFrame := &frame{Header: frameHeader{Type: "stream_end", ID: f.Header.ID}}
		writeMu.Lock()
		_ = writeFrame(r.conn, endFrame)
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

		endFrame := &frame{Header: frameHeader{Type: "stream_end", ID: f.Header.ID}}
		writeMu.Lock()
		_ = writeFrame(r.conn, endFrame)
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

func (r *Runtime) handleStreamData(f *frame) {
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

func (r *Runtime) handleStreamCancel(f *frame) {
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

	f := &frame{
		Header: frameHeader{Type: "stream_data", ID: s.id},
		Body:   b,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeFrame(s.conn, f)
}

func (s *workerStreamSink) Close() error {
	return nil
}

func (r *Runtime) dispatch(f *frame) *frame {
	var req abi.Request
	if err := json.Unmarshal(f.Body, &req); err != nil {
		errResp := abi.NewErrorResponse(fmt.Errorf("malformed request frame: %w", err))
		body, _ := json.Marshal(errResp)
		return &frame{Header: frameHeader{Type: "response", ID: f.Header.ID}, Body: body}
	}

	handlerID := abi.HandlerID(f.Header.Metadata["handler_id"])
	if handlerID == "" {
		handlerID = abi.HandlerID(req.RoutePath)
	}

	r.handlersMu.RLock()
	handler, ok := r.handlers[handlerID]
	r.handlersMu.RUnlock()

	if !ok {
		errResp := abi.NewErrorResponse(fmt.Errorf("handler %q not registered in worker", handlerID))
		errResp.Status = 404
		body, _ := json.Marshal(errResp)
		return &frame{Header: frameHeader{Type: "response", ID: f.Header.ID}, Body: body}
	}

	resp, err := handler(&req)
	if err != nil {
		resp = abi.NewErrorResponse(err)
	} else if resp == nil {
		resp = abi.NewResponse(204, nil)
	}

	respBytes, _ := json.Marshal(resp)
	return &frame{Header: frameHeader{Type: "response", ID: f.Header.ID}, Body: respBytes}
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
