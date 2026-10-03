// Package amqp answers: how are AMQP exchanges, routing keys, and queues handled?
package amqp

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

// Delivery represents an AMQP delivery message.
type Delivery struct {
	Exchange   string
	RoutingKey string
	Body       []byte
}

// Adapter implements transport.Adapter for AMQP.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	table      *router.Table
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized AMQP Adapter.
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
	return abi.TransportAMQP
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "amqp:exchange.routing_key"
}

// Normalize decodes a Delivery into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	d, ok := raw.(Delivery)
	if !ok {
		return nil, fmt.Errorf("expected Delivery, got %T", raw)
	}

	path := d.Exchange + "/" + d.RoutingKey
	req := abi.NewRequest(abi.TransportAMQP, "CONSUME", path)
	req.Topic = d.RoutingKey
	req.Body = d.Body
	req.Metadata["exchange"] = d.Exchange

	return req, nil
}

// Render writes an outbound payload.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// Serve initializes AMQP consumers.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table
	return nil
}

// Shutdown gracefully terminates the AMQP adapter.
func (a *Adapter) Shutdown(ctx context.Context) error {
	return nil
}
