package introspect

import (
	"encoding/json"
)

// RenderAsyncAPI produces an AsyncAPI fragment for pub/sub routes.
func RenderAsyncAPI(view *Response) ([]byte, error) {
	channels := make(map[string]any)

	for _, c := range view.Children {
		channels[c.Name] = map[string]any{
			"description": "PubSub channel: " + c.Name,
		}
	}

	doc := map[string]any{
		"asyncapi": "3.0.0",
		"info": map[string]any{
			"title":   "nxp AsyncAPI",
			"version": "1.0.0",
		},
		"channels": channels,
	}

	return json.MarshalIndent(doc, "", "  ")
}
