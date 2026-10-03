// Package ws provides developer-facing WebSocket context and duplex framing helpers.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the execution context for WebSocket handlers.
type Ctx struct {
	context.Context
	req      *abi.Request
	inChan   <-chan []byte
	outChan  chan<- []byte
}

// NewCtx constructs an initialized WebSocket Ctx.
func NewCtx(parent context.Context, req *abi.Request, in <-chan []byte, out chan<- []byte) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx{
		Context: parent,
		req:     req,
		inChan:  in,
		outChan: out,
	}
}

// Param returns a path parameter by name.
func (c *Ctx) Param(name string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(name)
}

// Query returns a query parameter value.
func (c *Ctx) Query(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.QueryParam(key)
}

// Header returns a header value.
func (c *Ctx) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}

// Identity returns the authenticated identity if present.
func (c *Ctx) Identity() *abi.Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}

// Bind unmarshals the initial request payload into target.
func (c *Ctx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}

// Next awaits the next inbound message frame from the client.
func (c *Ctx) Next() ([]byte, error) {
	select {
	case <-c.Done():
		return nil, c.Err()
	case msg, ok := <-c.inChan:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	}
}

// Write sends an outbound frame to the connected client.
func (c *Ctx) Write(data []byte) error {
	select {
	case <-c.Done():
		return c.Err()
	case c.outChan <- data:
		return nil
	}
}

// WriteJSON serializes v as JSON and transmits it to the client.
func (c *Ctx) WriteJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json frame: %w", err)
	}
	return c.Write(b)
}
