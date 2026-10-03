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
		for _, e := range selfEntries {
			if e.Route.Method != "" {
				methods = append(methods, e.Route.Method)
			}
		}

		resp.Self = &SelfView{
			Handler:   string(first.Route.ID),
			Methods:   methods,
			Auth:      first.Route.Auth,
			Scopes:    first.Route.Scopes,
			RateLimit: first.Route.RateLimit,
			Schema:    first.Route.Shape,
		}
	}

	// 2. Resolve direct children (one level only)
	children := table.Children(cleanPath)
	if children != nil {
		resp.Children = children
	}

	return resp
}
