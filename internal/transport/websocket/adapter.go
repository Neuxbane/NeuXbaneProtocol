// Package websocket answers: how are duplex WebSocket frames, heartbeats, and resume tokens handled?
package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/contract"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Session tracks connection state and resume tokens for reconnecting clients.
type Session struct {
	Token     string
	Path      string
	CreatedAt time.Time
}

// Adapter implements transport.Adapter for duplex WebSockets.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	server     *http.Server
	table      *router.Table
	sessions   map[string]*Session
	sessionsMu sync.RWMutex
}

// NewAdapter constructs an initialized WebSocket transport adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher: dispatcher,
		sessions:   make(map[string]*Session),
	}
}

// SetDispatcher sets the worker request dispatch callback.
func (a *Adapter) SetDispatcher(fn transport.DispatcherFunc) {
	a.dispatcher = fn
}

// Name implements transport.Adapter.
func (a *Adapter) Name() abi.Transport {
	return abi.TransportWebSocket
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "websocket:CONNECT:/path"
}

// Normalize transforms a raw *http.Request into an initial *abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	httpReq, ok := raw.(*http.Request)
	if !ok {
		return nil, fmt.Errorf("expected *http.Request, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportWebSocket, "CONNECT", httpReq.URL.Path)
	for k, v := range httpReq.Header {
		req.Headers[k] = v
	}
	for k, v := range httpReq.URL.Query() {
		req.Query[k] = v
	}

	// Extract identity from the Authorization Bearer header or the well-known
	// auth cookies (shared with the REST transport) so that guarded WebSocket
	// routes pass ingress contract validation and folder guards see the caller.
	req.Identity = transport.ExtractIdentity(httpReq)

	return req, nil
}

// Render writes a frame to an io.Writer.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// Serve accepts WebSocket connections on listener and routes them via table.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		a.handleUpgrade(w, r)
	})

	a.server = &http.Server{Handler: mux}
	return a.server.Serve(*listener)
}

func (a *Adapter) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	entry, params, found := a.table.Load(abi.TransportWebSocket, "CONNECT", r.URL.Path)
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

	// Ingress contract validation
	if valErr := contract.ValidateRequest(&entry.Route, abiReq); valErr != nil {
		http.Error(w, valErr.Error(), valErr.HTTPStatusCode())
		return
	}

	// Upgrade connection
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	// Assign or restore resume token
	token := r.URL.Query().Get("token")
	if token == "" {
		var tokenBytes [16]byte
		_, _ = rand.Read(tokenBytes[:])
		token = hex.EncodeToString(tokenBytes[:])
		a.sessionsMu.Lock()
		a.sessions[token] = &Session{Token: token, Path: r.URL.Path, CreatedAt: time.Now()}
		a.sessionsMu.Unlock()
	}

	// Send initial handshake frame with token
	_ = wsConn.WriteJSON(map[string]string{"type": "connected", "token": token})

	// Heartbeat setup
	wsConn.SetPongHandler(func(string) error {
		_ = wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping ticker
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			case <-stopChan:
				return
			}
		}
	}()

	// Read loop with backpressure
	for {
		_ = wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		msgType, msg, err := wsConn.ReadMessage()
		if err != nil {
			close(stopChan)
			return
		}

		if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
			// Each frame is dispatched as its own request. Carry over the
			// handshake context (headers, query, params, identity) so that
			// guards and handlers see the same authenticated caller that was
			// established during the upgrade. Without this, cookie/bearer
			// auth would succeed on the handshake but fail on every frame.
			//
			// The method must match the route's registered method ("CONNECT")
			// so the router resolves the same handler that served the upgrade.
			msgReq := abi.NewRequest(abi.TransportWebSocket, "CONNECT", r.URL.Path)
			msgReq.RoutePath = abiReq.RoutePath
			msgReq.Body = msg
			for k, v := range abiReq.Headers {
				msgReq.Headers[k] = v
			}
			for k, v := range abiReq.Query {
				msgReq.Query[k] = v
			}
			for k, v := range abiReq.Params {
				msgReq.Params[k] = v
			}
			msgReq.Identity = abiReq.Identity

			if a.dispatcher != nil {
				resp, err := a.dispatcher(r.Context(), msgReq)
				if err == nil && resp != nil && len(resp.Body) > 0 {
					_ = wsConn.WriteMessage(websocket.TextMessage, resp.Body)
				}
			}
		}
	}
}

// Shutdown stops the WebSocket server.
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}

// Handler returns an http.Handler that serves WebSocket upgrade requests using
// the adapter's route table. It lets a host server (e.g. the REST adapter)
// delegate upgrade requests so both transports share one listener/port.
func (a *Adapter) Handler(table *router.Table) http.Handler {
	a.table = table
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.handleUpgrade(w, r)
	})
}
