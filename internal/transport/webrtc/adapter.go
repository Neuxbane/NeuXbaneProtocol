// Package webrtc answers: how are WebRTC signaling, data channels, and media sessions initiated and managed?
package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/sfu"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Adapter implements transport.Adapter for WebRTC signaling and media channels.
type Adapter struct {
	dispatcher       transport.DispatcherFunc
	streamDispatcher transport.StreamDispatcherFunc
	server           *http.Server
	table            *router.Table
	mu               sync.RWMutex
}

// NewAdapter constructs an initialized WebRTC Adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher: dispatcher,
	}
}

// SetDispatcher sets the worker dispatcher callback.
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
	return abi.TransportWebRTC
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "webrtc:SIGNAL:/room"
}

// Normalize parses SDP offer requests into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	httpReq, ok := raw.(*http.Request)
	if !ok {
		return nil, fmt.Errorf("expected *http.Request for webrtc signaling, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportWebRTC, "SIGNAL", httpReq.URL.Path)
	if httpReq.Body != nil {
		body, _ := io.ReadAll(httpReq.Body)
		_ = httpReq.Body.Close()
		req.Body = body
	}
	return req, nil
}

// Render transmits SDP answers or media descriptions back to client.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	rw, isHTTP := w.(http.ResponseWriter)
	if isHTTP {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
	}
	_, err := w.Write(resp.Body)
	return err
}

// Serve handles signaling HTTP endpoints for WebRTC negotiation.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := a.Normalize(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		roomName := r.URL.Query().Get("room")
		if roomName == "" {
			roomName = "default"
		}

		peerID := fmt.Sprintf("peer-%d", r.Context().Value("trace_id"))
		peer := sfu.GlobalEngine.NewPeer(peerID)
		room := sfu.GlobalEngine.GetOrCreateRoom(roomName)
		room.AddPeer(peer)

		if a.dispatcher != nil {
			resp, err := a.dispatcher(r.Context(), req)
			if err == nil && resp != nil {
				_ = a.Render(w, resp, transport.RenderCtx{Raw: r})
				return
			}
		}

		// Fallback default signaling response
		answer := map[string]string{
			"type": "answer",
			"room": roomName,
			"peer": peerID,
		}
		b, _ := json.Marshal(answer)
		resp := abi.NewResponse(200, b)
		_ = a.Render(w, resp, transport.RenderCtx{Raw: r})
	})

	a.server = &http.Server{Handler: mux}
	return a.server.Serve(*listener)
}

// Shutdown stops the WebRTC signaling server.
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}
