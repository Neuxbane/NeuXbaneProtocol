// Package rest provides the developer-facing HTTP/REST context and helper methods.
package rest

import (
	"context"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Ctx is the handler execution context provided to REST route handlers.
type Ctx struct {
	context.Context
	req *abi.Request
}

// NewCtx creates an initialized REST Ctx.
func NewCtx(parent context.Context, req *abi.Request) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	return &Ctx{
		Context: parent,
		req:     req,
	}
}

// Request returns the underlying normalized abi.Request.
func (c *Ctx) Request() *abi.Request {
	return c.req
}

// Param returns a path parameter by name.
func (c *Ctx) Param(name string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Param(name)
}

// Query returns the query parameter value for key.
func (c *Ctx) Query(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.QueryParam(key)
}

// Header returns the request header value for key.
func (c *Ctx) Header(key string) string {
	if c.req == nil {
		return ""
	}
	return c.req.Header(key)
}

// Identity returns the authenticated caller context if present.
func (c *Ctx) Identity() *abi.Identity {
	if c.req == nil {
		return nil
	}
	return c.req.Identity
}

// Bind unmarshals the request payload into target.
func (c *Ctx) Bind(target any) error {
	if c.req == nil {
		return nil
	}
	return c.req.BindJSON(target)
}
