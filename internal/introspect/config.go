// Package introspect answers: what routes, handlers, and schemas exist at a given path in this runtime?
package introspect

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
)

// Config re-exports config.IntrospectConfig.
type Config = config.IntrospectConfig

// SelfView represents the handler at the current path level.
//
// The schema contract is keyed by method: the same path served by different
// methods (e.g. GET vs POST) has distinct input and output schemas, so a single
// schema field cannot represent it. Schemas maps each method to its route Shape.
type SelfView struct {
	Handler      string               `json:"handler"`
	Methods      []string             `json:"methods"`
	Auth         string               `json:"auth,omitempty"`
	Scopes       []string             `json:"scopes,omitempty"`
	RateLimit    *abi.RateLimitConfig `json:"ratelimit,omitempty"`
	Schemas      map[string]any       `json:"schemas"`
	Descriptions map[string]string    `json:"descriptions,omitempty"`
}
