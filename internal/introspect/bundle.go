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
		if e.Route.Shape != nil {
			schemas[string(e.Route.ID)] = e.Route.Shape
		}
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
