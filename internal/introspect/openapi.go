package introspect

import (
	"encoding/json"
	"strings"
)

// RenderOpenAPI produces an OpenAPI 3.1 fragment for the current route level.
func RenderOpenAPI(view *Response) ([]byte, error) {
	paths := make(map[string]map[string]any)
	pathItem := make(map[string]any)

	if view.Self != nil {
		for _, m := range view.Self.Methods {
			methodLower := strings.ToLower(m)
			pathItem[methodLower] = map[string]any{
				"operationId": view.Self.Handler + "." + m,
				"summary":     "Handler for " + view.Path,
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Successful operation",
					},
				},
			}
		}
	}

	paths[view.Path] = pathItem

	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "nxp API",
			"version": "1.0.0",
		},
		"paths": paths,
	}

	return json.MarshalIndent(doc, "", "  ")
}
