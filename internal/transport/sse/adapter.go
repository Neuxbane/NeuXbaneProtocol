// Package sse answers: how are Server-Sent Events (SSE) streams and server-push events handled over HTTP?
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/contract"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Adapter implements transport.Adapter for Server-Sent Events (SSE).
type Adapter struct {
	dispatcher       transport.DispatcherFunc
	streamDispatcher transport.StreamDispatcherFunc
	server           *http.Server
	table            *router.Table
	mu               sync.RWMutex
}

// NewAdapter constructs an initialized SSE transport adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher: dispatcher,
	}
}

// SetDispatcher sets the worker request dispatch callback.
func (a *Adapter) SetDispatcher(fn transport.DispatcherFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dispatcher = fn
}

// SetStreamDispatcher sets the streaming request dispatch callback.
func (a *Adapter) SetStreamDispatcher(fn transport.StreamDispatcherFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.streamDispatcher = fn
}

// Name implements transport.Adapter.
func (a *Adapter) Name() abi.Transport {
	return abi.TransportSSE
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "sse:GET:/events"
}

// Normalize transforms a raw *http.Request into a normalized *abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	httpReq, ok := raw.(*http.Request)
	if !ok {
		return nil, fmt.Errorf("expected *http.Request, got %T", raw)
	}

	method := httpReq.Method
	if method == "" {
		method = "GET"
	}
	req := abi.NewRequest(abi.TransportSSE, method, httpReq.URL.Path)
	for k, v := range httpReq.Header {
		req.Headers[k] = v
	}
	for k, v := range httpReq.URL.Query() {
		req.Query[k] = v
	}
	req.Identity = transport.ExtractIdentity(httpReq)
	return req, nil
}

// Render writes an SSE response or frame to an io.Writer.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	rw, isHTTP := w.(http.ResponseWriter)
	if isHTTP {
		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		rw.Header().Set("X-Accel-Buffering", "no")
		status := resp.Status
		if status == 0 {
			status = http.StatusOK
		}
		rw.WriteHeader(status)
	}

	if len(resp.Body) == 0 {
		return nil
	}

	// If the body already starts with "data:" or "event:", write as-is
	if len(resp.Body) >= 5 && string(resp.Body[:5]) == "data:" {
		_, err := w.Write(resp.Body)
		return err
	}

	_, err := fmt.Fprintf(w, "data: %s\n\n", resp.Body)
	return err
}

// Serve starts listening for SSE HTTP traffic on listener.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		a.handleSSE(w, r)
	})

	a.server = &http.Server{Handler: mux}
	return a.server.Serve(*listener)
}

// Handler returns an http.Handler that serves SSE requests using the adapter's route table.
func (a *Adapter) Handler(table *router.Table) http.Handler {
	a.table = table
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.handleSSE(w, r)
	})
}

// Shutdown stops the SSE server.
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}

type sseSink struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	closed  bool
}

func (s *sseSink) Send(data any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("sse sink closed")
	}

	var payload []byte
	switch v := data.(type) {
	case []byte:
		payload = v
	case string:
		payload = []byte(v)
	default:
		var err error
		payload, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal sse payload: %w", err)
		}
	}

	if len(payload) >= 5 && string(payload[:5]) == "data:" {
		if _, err := s.w.Write(payload); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(s.w, "data: %s\n\n", payload); err != nil {
			return err
		}
	}

	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

func (s *sseSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (a *Adapter) handleSSE(w http.ResponseWriter, r *http.Request) {
	entry, params, found := a.table.Load(abi.TransportSSE, r.Method, r.URL.Path)
	if !found {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}

	abiReq, err := a.Normalize(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	abiReq.RoutePath = entry.Route.Path
	abiReq.Params = params

	if valErr := contract.ValidateRequest(&entry.Route, abiReq); valErr != nil {
		http.Error(w, valErr.Error(), valErr.HTTPStatusCode())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	a.mu.RLock()
	streamDispatcher := a.streamDispatcher
	dispatcher := a.dispatcher
	a.mu.RUnlock()

	sink := &sseSink{w: w, flusher: flusher}
	defer sink.Close()

	if streamDispatcher != nil {
		inChan := make(chan []byte)
		defer close(inChan)
		_ = streamDispatcher(r.Context(), abiReq, sink, inChan)
		return
	}

	if dispatcher != nil {
		resp, err := dispatcher(r.Context(), abiReq)
		if err == nil && resp != nil {
			_ = sink.Send(resp.Body)
		}
	}
}

