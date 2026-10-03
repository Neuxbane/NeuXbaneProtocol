// Package udp answers: how are connectionless UDP datagrams received, routed, and responded to?
package udp

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/transport"
)

func init() {
	transport.RegisterAdapter(NewAdapter(nil))
}

// RawDatagram encapsulates a received UDP packet with its sender address.
type RawDatagram struct {
	Addr net.Addr
	Data []byte
}

// Adapter implements transport.Adapter for UDP datagrams.
type Adapter struct {
	dispatcher transport.DispatcherFunc
	conn       net.PacketConn
	table      *router.Table
	closed     atomic.Bool
	mu         sync.RWMutex
}

// NewAdapter constructs an initialized UDP Adapter.
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
	return abi.TransportUDP
}

// MatchKey implements transport.Adapter.
func (a *Adapter) MatchKey() string {
	return "udp:SEND:/path"
}

// Normalize decodes a RawDatagram into an abi.Request.
func (a *Adapter) Normalize(raw any) (*abi.Request, error) {
	dg, ok := raw.(RawDatagram)
	if !ok {
		return nil, fmt.Errorf("expected RawDatagram, got %T", raw)
	}

	req := abi.NewRequest(abi.TransportUDP, "SEND", "/")
	req.Body = dg.Data
	if dg.Addr != nil {
		req.Metadata["remote_addr"] = dg.Addr.String()
	}

	return req, nil
}

// Render writes a datagram to an io.Writer.
func (a *Adapter) Render(w io.Writer, resp *abi.Response, ctx transport.RenderCtx) error {
	_, err := w.Write(resp.Body)
	return err
}

// Serve starts listening for UDP datagrams on listener.
func (a *Adapter) Serve(listener *net.Listener, table *router.Table) error {
	a.table = table

	// Use listener's local address to bind UDP packet conn
	lAddr := (*listener).Addr().String()
	pConn, err := net.ListenPacket("udp", lAddr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", lAddr, err)
	}
	a.conn = pConn

	buf := make([]byte, 65535)
	for {
		n, remote, err := pConn.ReadFrom(buf)
		if err != nil {
			if a.closed.Load() {
				return nil
			}
			return err
		}

		payload := make([]byte, n)
		copy(payload, buf[:n])

		go a.handleDatagram(remote, payload)
	}
}

func (a *Adapter) handleDatagram(remote net.Addr, data []byte) {
	req := abi.NewRequest(abi.TransportUDP, "SEND", "/")
	req.Body = data
	req.Metadata["remote_addr"] = remote.String()

	entry, _, found := a.table.Load(abi.TransportUDP, "SEND", "/")
	if !found {
		return
	}

	// Check datagram response mode
	mode := abi.DatagramResponseEcho
	if dgShape, ok := entry.Route.Shape.(abi.DatagramShape); ok {
		if dgShape.MaxSize > 0 && len(data) > dgShape.MaxSize {
			return
		}
		mode = dgShape.Response
	}

	if mode == abi.DatagramResponseNone {
		if a.dispatcher != nil {
			_, _ = a.dispatcher(context.Background(), req)
		}
		return
	}

	if mode == abi.DatagramResponseEcho {
		_, _ = a.conn.WriteTo(data, remote)
		return
	}

	// Unicast response mode
	if a.dispatcher != nil {
		resp, err := a.dispatcher(context.Background(), req)
		if err == nil && resp != nil && len(resp.Body) > 0 {
			_, _ = a.conn.WriteTo(resp.Body, remote)
		}
	}
}

// Shutdown gracefully terminates the UDP packet connection.
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.closed.CompareAndSwap(false, true) {
		if a.conn != nil {
			return a.conn.Close()
		}
	}
	return nil
}
