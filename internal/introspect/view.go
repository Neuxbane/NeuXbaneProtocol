package introspect

import (
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
)

// Response is the canonical JSON representation for ?nxp introspection.
type Response struct {
	Path     string         `json:"path"`
	Self     *SelfView      `json:"self"`
	Children []router.Child `json:"children"`
}

// BuildView creates a single-level introspection view for the given path from the routing table.
func BuildView(table *router.Table, path string) *Response {
	cleanPath := path
	if cleanPath == "" {
		cleanPath = "/"
	}
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	if len(cleanPath) > 1 && strings.HasSuffix(cleanPath, "/") {
		cleanPath = strings.TrimSuffix(cleanPath, "/")
	}

	resp := &Response{
		Path:     cleanPath,
		Children: make([]router.Child, 0),
	}

	// 1. Resolve self handler
	selfEntries := table.SelfEntries(cleanPath)
	if len(selfEntries) > 0 {
		first := selfEntries[0]
		methods := make([]string, 0, len(selfEntries))
		schemas := make(map[string]any, len(selfEntries))
		descriptions := make(map[string]string, len(selfEntries))
		for _, e := range selfEntries {
			if e.Route.Method == "" {
				continue
			}
			methods = append(methods, e.Route.Method)
			// The schema contract is keyed by method: each method on the same
			// path carries its own request/response shape.
			if e.Route.Shape != nil {
				schemas[e.Route.Method] = e.Route.Shape
			}
			if e.Route.Description != "" {
				descriptions[e.Route.Method] = e.Route.Description
			}
		}

		resp.Self = &SelfView{
			Handler:      string(first.Route.ID),
			Methods:      methods,
			Auth:         first.Route.Auth,
			Scopes:       first.Route.Scopes,
			RateLimit:    first.Route.RateLimit,
			Schemas:      schemas,
			Descriptions: descriptions,
		}
	}

	// 2. Resolve direct children (one level only)
	children := table.Children(cleanPath)
	if children != nil {
		resp.Children = children
	}

	return resp
}
