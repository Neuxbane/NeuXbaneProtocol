// Package nats answers: how are NATS subjects, queue groups, and JetStream messages routed?
package nats

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

// Msg represents a NATS inbound message.
type Msg struct {
	Subject string
	Data    []byte
	Reply   string
}

// Adapter implements transport.Adapter for NATS.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	table      *router.Table
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized NATS Adapter.
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
	return abi.TransportNATS
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "nats:subject.>"
}

// Normalize decodes a Msg into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	msg, ok := raw.(Msg)
	if !ok {
		return nil, fmt.Errorf("expected Msg, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportNATS, "SUB", msg.Subject)
	req.Topic = msg.Subject
	req.Body = msg.Data
	if msg.Reply != "" {
		req.Metadata["reply"] = msg.Reply
	}

	return req, nil
}

// Render writes an outbound payload to an io.Writer.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// Serve initializes NATS subscribers.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table
	return nil
}

// Shutdown gracefully stops the NATS adapter.
func (a *Adapter) Shutdown(ctx context.Context) error {
	return nil
}
