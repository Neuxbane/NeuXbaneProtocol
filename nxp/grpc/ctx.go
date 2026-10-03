// Package grpc provides developer-facing gRPC context and rpc helpers.
package grpc

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the execution context for gRPC handlers.
type Ctx struct {
	context.Context
	req *abi.Request
}

// NewCtx constructs an initialized gRPC Ctx.
func NewCtx(parent context.Context, req *abi.Request) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx{
		Context: parent,
		req:     req,
	}
}

// Param returns a path parameter.
func (c *Ctx) Param(name string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(name)
}

// Header returns gRPC metadata entry by key.
func (c *Ctx) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}

// Identity returns authenticated caller identity.
func (c *Ctx) Identity() *abi.Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}

// Bind unmarshals the inbound payload into target.
func (c *Ctx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}
