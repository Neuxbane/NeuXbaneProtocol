// Package kafka provides developer-facing Kafka context, topic, and partition consumer helpers.
package kafka

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the execution context for Kafka stream handlers.
type Ctx[In, Out any] struct {
	context.Context
	req      *abi.Request
	committer func() error
}

// NewCtx constructs an initialized Kafka Ctx.
func NewCtx[In, Out any](parent context.Context, req *abi.Request, committer func() error) *Ctx[In, Out] {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx[In, Out]{
		Context:   parent,
		req:       req,
		committer: committer,
	}
}

// Topic returns the Kafka topic name.
func (c *Ctx[In, Out]) Topic() string {
	if c.req == nil {
		return ""
	}
	return c.req.Topic
}

// Partition returns the Kafka partition ID.
func (c *Ctx[In, Out]) Partition() int {
	if c.req == nil {
		return 0
	}
	return c.req.Partition
}

// Offset returns the Kafka record offset.
func (c *Ctx[In, Out]) Offset() int64 {
	if c.req == nil {
		return 0
	}
	return c.req.Offset
}

// Payload unmarshals and returns the typed record value.
func (c *Ctx[In, Out]) Payload() (In, error) {
	var in In
	if c.req == nil {
		return in, nil
	}
	err := c.req.BindJSON(&in)
	return in, err
}

// Commit commits the current partition offset.
func (c *Ctx[In, Out]) Commit() error {
	if c.committer != nil {
		return c.committer()
	}
	return nil
}
