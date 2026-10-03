// Package grpc answers: how are gRPC HTTP/2 framed RPC calls received, normalized, and rendered?
package grpc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Adapter implements transport.Adapter for gRPC.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	server     *http.Server
	table      *router.Table
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized gRPC Adapter.
func NewAdapter(dispatcher transport.DispatcherFunc) *Adapter {
	return &Adapter{
		dispatcher: dispatcher,
	}
}

// SetDispatcher sets the worker dispatcher callback.
func (a *Adapter) SetDispatcher(fn transport.DispatcherFunc) {
	a.dispatcher = fn
}

// Name implements transport.Adapter.
func (a *Adapter) Name() abi.Transport {
	return abi.TransportGRPC
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "grpc:POST:/service/method"
}

// Normalize decodes a gRPC HTTP/2 request into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	httpReq, ok := raw.(*http.Request)
	if !ok {
		return nil, fmt.Errorf("expected *http.Request, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportGRPC, "POST", httpReq.URL.Path)
	for k, v := range httpReq.Header {
		req.Headers[k] = v
	}

	if httpReq.Body != nil {
		bodyBytes, err := io.ReadAll(httpReq.Body)
		if err != nil {
			return nil, err
		}
		_ = httpReq.Body.Close()

		// Unframe gRPC payload (5-byte prefix: 1-byte compression flag + 4-byte BE length)
		if len(bodyBytes) >= 5 {
			msgLen := binary.BigEndian.Uint32(bodyBytes[1:5])
			if int(msgLen) <= len(bodyBytes)-5 {
				req.Body = bodyBytes[5 : 5+msgLen]
			} else {
				req.Body = bodyBytes[5:]
			}
		} else {
			req.Body = bodyBytes
		}
	}

	auth := httpReq.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		req.Identity = &abi.Identity{Subject: token}
	}

	return req, nil
}

// Render frames and writes an outbound gRPC payload with standard 5-byte header.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	rw, isHTTP := w.(http.ResponseWriter)
	if isHTTP {
		rw.Header().Set("Content-Type", "application/grpc")
		rw.Header().Set("grpc-status", "0")
		rw.WriteHeader(http.StatusOK)
	}

	// 5-byte gRPC framing
	frame := make([]byte, 5+len(resp.Body))
	frame[0] = 0 // uncompressed
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(resp.Body)))
	copy(frame[5:], resp.Body)

	_, err := w.Write(frame)
	return err
}

// Serve starts listening for gRPC requests.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := a.Normalize(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if a.dispatcher != nil {
			resp, err := a.dispatcher(r.Context(), req)
			if err != nil {
				w.Header().Set("grpc-status", "13") // Internal
				return
			}
			_ = a.Render(w, resp, transport.RenderCtx{Raw: r})
		}
	})

	a.server = &http.Server{Handler: mux}
	return a.server.Serve(*listener)
}

// Shutdown gracefully terminates the gRPC server.
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}
