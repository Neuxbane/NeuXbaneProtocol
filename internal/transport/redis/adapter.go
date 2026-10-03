// Package redis answers: how are Redis Pub/Sub channels and Redis Streams consumer groups integrated?
package redis

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// Message represents a Redis Pub/Sub or Stream message.
type Message struct {
	Channel string
	Payload []byte
	Stream  string
	ID      string
}

// Adapter implements transport.Adapter for Redis.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	table      *router.Table
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized Redis Adapter.
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
	return abi.TransportRedis
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "redis:channel"
}

// Normalize decodes a Redis Message into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	msg, ok := raw.(Message)
	if !ok {
		return nil, fmt.Errorf("expected Message, got %T", raw)
	}

	target := msg.Channel
	if target == "" {
		target = msg.Stream
	}

	req := abi.NewRequest(abi.TransportRedis, "SUB", target)
	req.Topic = target
	req.Body = msg.Payload
	if msg.ID != "" {
		req.Metadata["stream_id"] = msg.ID
	}

	return req, nil
}

// Render writes an outbound payload.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// Serve initializes Redis subscribers.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table
	return nil
}

// Shutdown gracefully terminates the Redis adapter.
func (a *Adapter) Shutdown(ctx context.Context) error {
	return nil
}
