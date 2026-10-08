// Package transport answers: how are network and messaging protocols adapted to the unified nxp request/response pipeline?
package transport

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
)

// RenderCtx provides rendering context for transport response serialization.
type RenderCtx struct {
	Headers map[string]string
	Format  string
	Raw     any
}

// Adapter defines the uniform transport adapter contract.
type Adapter interface {
	Name() abi.Transport
	MatchKey() string
	Normalize(raw any) (*abi.Request, error)
	Render(w io.Writer, resp *abi.Response, ctx RenderCtx) error
	Serve(listener *net.Listener, table *router.Table) error
	Shutdown(ctx context.Context) error
}

var (
	adaptersMu sync.RWMutex
	adapters   = make(map[abi.Transport]Adapter)
)

// RegisterAdapter registers a transport adapter globally.
func RegisterAdapter(a Adapter) {
	adaptersMu.Lock()
	defer adaptersMu.Unlock()
	adapters[a.Name()] = a
}

// GetAdapter retrieves a registered transport adapter by name.
func GetAdapter(name abi.Transport) (Adapter, bool) {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()
	a, ok := adapters[name]
	return a, ok
}

// AllAdapters returns a slice of all registered transport adapters.
func AllAdapters() []Adapter {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()
	list := make([]Adapter, 0, len(adapters))
	for _, a := range adapters {
		list = append(list, a)
	}
	return list
}

// DispatcherFunc defines the handler invocation callback for adapters.
type DispatcherFunc func(ctx context.Context, req *abi.Request) (*abi.Response, error)

// StreamSink defines an outbound event/message sink for streaming transports.
type StreamSink = abi.StreamSink

// StreamDispatcherFunc defines the streaming handler invocation callback for adapters.
type StreamDispatcherFunc func(ctx context.Context, req *abi.Request, sink StreamSink, in <-chan []byte) error

