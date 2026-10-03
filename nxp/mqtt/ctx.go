// Package mqtt provides developer-facing MQTT context and PubSub helpers.
package mqtt

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the execution context for MQTT pub/sub handlers.
type Ctx[In, Out any] struct {
	context.Context
	req       *abi.Request
	publisher func(topic string, payload []byte) error
}

// NewCtx constructs an initialized MQTT Ctx.
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

// Topic returns the MQTT topic on which the message arrived.
func (c *Ctx[In, Out]) Topic() string {
	if c.req == nil {
		return ""
	}
	return c.req.Topic
}

// Payload unmarshals and returns the typed input message.
func (c *Ctx[In, Out]) Payload() (In, error) {
	var in In
	if c.req == nil {
		return in, nil
	}
	err := c.req.BindJSON(&in)
	return in, err
}

// Publish publishes an outbound message to a target MQTT topic.
func (c *Ctx[In, Out]) Publish(topic string, msg []byte) error {
	if c.publisher != nil {
		return c.publisher(topic, msg)
	}
	return nil
}

// Ack acknowledges receipt of QoS 1 or 2 messages.
func (c *Ctx[In, Out]) Ack() error {
	return nil
}

// Nack rejects message delivery.
func (c *Ctx[In, Out]) Nack() error {
	return nil
}

// Header returns metadata associated with the MQTT message.
func (c *Ctx[In, Out]) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}

// Identity returns the authenticated publisher identity if present.
func (c *Ctx[In, Out]) Identity() *abi.Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}
