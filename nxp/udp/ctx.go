// Package udp provides developer-facing UDP context and datagram helpers.
package udp

import (
	"context"
	"net"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the execution context for UDP datagram handlers.
type Ctx struct {
	context.Context
	req        *abi.Request
	remoteAddr net.Addr
	responder  func([]byte) error
}

// NewCtx constructs an initialized UDP Ctx.
func NewCtx(parent context.Context, req *abi.Request, remote net.Addr, responder func([]byte) error) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx{
		Context:    parent,
		req:        req,
		remoteAddr: remote,
		responder:  responder,
	}
}

// Data returns the raw datagram payload bytes.
func (c *Ctx) Data() []byte {
	if c.req == nil {
		return nil
	}
	return c.req.Body
}

// RemoteAddr returns the client network address.
func (c *Ctx) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Write sends an outbound datagram response to the client.
func (c *Ctx) Write(data []byte) error {
	if c.responder != nil {
		return c.responder(data)
	}
	return nil
}

// Bind unmarshals the datagram JSON payload into target.
func (c *Ctx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}
