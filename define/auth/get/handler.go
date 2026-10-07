// Package auth handles authentication operations.
// @desc Get the caller profile status
// @auth required
// @ratelimit 20 40
package get

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"
)

// IndexResult returns the caller profile status.
type IndexResult struct {
	UserID string `json:"user_id" validate:"required"`
	Role   string `json:"role" validate:"required"`
}

// Handler handles GET /auth (index handler appearing as self).
func Handler(ctx *rest.Ctx) (IndexResult, error) {
	sub := "anonymous"
	if ctx.Identity() != nil && ctx.Identity().Subject != "" {
		sub = ctx.Identity().Subject
	}
	return IndexResult{
		UserID: sub,
		Role:   "member",
	}, nil
}
