package mqtt

import (
	"context"

	"abi"
)

type Ctx[In, Out any] struct {
	context.Context
	req     *abi.Request
	Payload In
}

func NewCtx[In, Out any](parent context.Context, req *abi.Request, client any) *Ctx[In, Out] {
	if parent == nil {
		parent = context.Background()
	}
	var in In
	if req != nil && len(req.Body) > 0 {
		_ = req.BindJSON(&in)
	}
	return &Ctx[In, Out]{Context: parent, req: req, Payload: in}
}

func (c *Ctx[In, Out]) Request() *abi.Request { return c.req }
func (c *Ctx[In, Out]) Topic() string {
	if c.req == nil {
		return ""
	}
	return c.req.Topic
}
