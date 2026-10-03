// Package introspect answers: what routes, handlers, and schemas exist at a given path in this runtime?
package introspect

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
)

// Config re-exports config.IntrospectConfig.
type Config = config.IntrospectConfig

// SelfView represents the handler at the current path level.
type SelfView struct {
	Handler   string               `json:"handler"`
	Methods   []string             `json:"methods"`
	Auth      string               `json:"auth,omitempty"`
	Scopes    []string             `json:"scopes,omitempty"`
	RateLimit *abi.RateLimitConfig `json:"ratelimit,omitempty"`
	Schema    any                  `json:"schema,omitempty"`
}
