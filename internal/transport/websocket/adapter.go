// Package websocket answers: how are duplex WebSocket frames, heartbeats, and resume tokens handled?
package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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
	dispatcher       transport.DispatcherFunc
	streamDispatcher transport.StreamDispatcherFunc
	server           *http.Server
	table            *router.Table
	sessions         map[string]*Session
	sessionsMu       sync.RWMutex
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

// SetStreamDispatcher sets the streaming request dispatch callback.
func (a *Adapter) SetStreamDispatcher(fn transport.StreamDispatcherFunc) {
	a.streamDispatcher = fn
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

	connCtx, cancelConn := context.WithCancel(r.Context())
	defer cancelConn()

	type wsOutMsg struct {
		msgType int
		data    []byte
	}

	outChan := make(chan wsOutMsg, 256)
	var writeClosed atomic.Bool
	var writeWg sync.WaitGroup
	writeWg.Add(1)

	// Outbound write pump goroutine ensuring thread-safe serialized writes to wsConn
	go func() {
		defer writeWg.Done()
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-outChan:
				if !ok {
					_ = wsConn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				_ = wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := wsConn.WriteMessage(msg.msgType, msg.data); err != nil {
					return
				}
			case <-ticker.C:
				_ = wsConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			case <-connCtx.Done():
				return
			}
		}
	}()

	writeOut := func(msgType int, data []byte) error {
		if writeClosed.Load() {
			return fmt.Errorf("websocket connection closed")
		}
		select {
		case <-connCtx.Done():
			return connCtx.Err()
		case outChan <- wsOutMsg{msgType: msgType, data: data}:
			return nil
		}
	}

	// Send initial handshake frame with token
	connectedBytes, _ := json.Marshal(map[string]string{"type": "connected", "token": token})
	_ = writeOut(websocket.TextMessage, connectedBytes)

	// Heartbeat setup
	wsConn.SetPongHandler(func(string) error {
		_ = wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	isStream := entry.Route.Shape != nil && (entry.Route.Shape.ShapeKind() == abi.ShapeKindStream || strings.HasSuffix(string(entry.Route.ID), ".stream") || strings.Contains(entry.Route.Path, "stream"))
	clientInChan := make(chan []byte, 64)

	sink := &wsStreamSink{
		sendFn: func(data any) error {
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
			return writeOut(websocket.TextMessage, b)
		},
	}

	// For streaming routes, invoke the handler immediately on connection upgrade
	if isStream {
		go func() {
			if a.streamDispatcher != nil {
				_ = a.streamDispatcher(connCtx, abiReq, sink, clientInChan)
			} else if a.dispatcher != nil {
				resp, err := a.dispatcher(connCtx, abiReq)
				if err == nil && resp != nil && len(resp.Body) > 0 {
					_ = writeOut(websocket.TextMessage, resp.Body)
				}
			}
		}()
	}

	// Inbound read loop
	for {
		_ = wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		msgType, msg, err := wsConn.ReadMessage()
		if err != nil {
			cancelConn()
			break
		}

		if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
			if isStream {
				select {
				case clientInChan <- msg:
				case <-connCtx.Done():
					break
				}
			} else {
				// Each frame is dispatched as its own request for frame-by-frame RPC routes.
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
					resp, err := a.dispatcher(connCtx, msgReq)
					if err == nil && resp != nil && len(resp.Body) > 0 {
						_ = writeOut(websocket.TextMessage, resp.Body)
					}
				}
			}
		}
	}

	writeClosed.Store(true)
	close(outChan)
	writeWg.Wait()
}

type wsStreamSink struct {
	sendFn func(data any) error
}

func (s *wsStreamSink) Send(data any) error {
	return s.sendFn(data)
}

func (s *wsStreamSink) Close() error {
	return nil
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
