// Package nats provides developer-facing NATS context and subject messaging helpers.
package nats

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the execution context for NATS subject handlers.
type Ctx[In, Out any] struct {
	context.Context
	req       *abi.Request
	publisher func(subj string, payload []byte) error
}

// NewCtx constructs an initialized NATS Ctx.
func NewCtx[In, Out any](parent context.Context, req *abi.Request, pub func(string, []byte) error) *Ctx[In, Out] {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx[In, Out]{
		Context:   parent,
		req:       req,
		publisher: pub,
	}
}

// Subject returns the NATS subject on which the message arrived.
func (c *Ctx[In, Out]) Subject() string {
	if c.req == nil {
		return ""
	}
	return c.req.Topic
}

// Payload unmarshals and returns the typed input payload.
func (c *Ctx[In, Out]) Payload() (In, error) {
	var in In
	if c.req == nil {
		return in, nil
	}
	err := c.req.BindJSON(&in)
	return in, err
}

// Publish sends an outbound message to a target NATS subject.
func (c *Ctx[In, Out]) Publish(subject string, msg []byte) error {
	if c.publisher != nil {
		return c.publisher(subject, msg)
	}
	return nil
}
