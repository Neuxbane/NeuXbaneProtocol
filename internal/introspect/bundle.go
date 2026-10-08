package introspect

import (
	"encoding/json"
	"net/http"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
)

// HandleSchemaBundle serves the aggregated schema bundle for the build at /__nxp/schema.
func HandleSchemaBundle(w http.ResponseWriter, r *http.Request, table *router.Table, buildID string) bool {
	if r.URL.Path != "/__nxp/schema" {
		return false
	}

	w.Header().Set("Content-Type", "application/schema+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	entries := table.AllEntries()
	schemas := make(map[string]any)

	for _, e := range entries {
		if e.Route.Shape == nil {
			continue
		}
		// Marshal the shape to a generic map so we can attach route-level
		// metadata (the description) alongside the request/response schemas.
		raw, err := json.Marshal(e.Route.Shape)
		if err != nil {
			continue
		}
		def := make(map[string]any)
		if err := json.Unmarshal(raw, &def); err != nil {
			continue
		}
		if e.Route.Description != "" {
			def["description"] = e.Route.Description
		}
		// Surface route metadata (e.g. the declarative stream poll contract)
		// so generic consumers can drive the route without endpoint-specific
		// code. Keys are namespaced (e.g. "stream.action").
		if len(e.Route.Metadata) > 0 {
			def["metadata"] = e.Route.Metadata
		}
		schemas[string(e.Route.ID)] = def
	}

	bundle := map[string]any{
		"$schema":     abi.JSONSchema202012URI,
		"build_id":    buildID,
		"buildId":     buildID,
		"definitions": schemas,
	}

	_ = json.NewEncoder(w).Encode(bundle)
	return true
}
