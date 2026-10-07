package introspect

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// RenderOpenAPI produces an OpenAPI 3.1 fragment for the current route level.
func RenderOpenAPI(view *Response) ([]byte, error) {
	paths := make(map[string]map[string]any)
	pathItem := make(map[string]any)

	if view.Self != nil {
		for _, m := range view.Self.Methods {
			methodLower := strings.ToLower(m)
			op := map[string]any{
				"operationId": view.Self.Handler + "." + m,
				"summary":     "Handler for " + view.Path,
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Successful operation",
					},
				},
			}

			// Attach the per-method schema contract. The same path served by
			// different methods has distinct request/response shapes.
			if shape, ok := view.Self.Schemas[m]; ok && shape != nil {
				if rr, ok := shape.(abi.RequestResponseShape); ok {
					if rr.Request != nil {
						op["requestBody"] = map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": rr.Request,
								},
							},
						}
					}
					if len(rr.Responses) > 0 {
						responses := make(map[string]any, len(rr.Responses))
						for code, s := range rr.Responses {
							responses[strconv.Itoa(code)] = map[string]any{
								"description": "Response",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": s,
									},
								},
							}
						}
						op["responses"] = responses
					}
				}
			}

			pathItem[methodLower] = op
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
