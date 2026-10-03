package introspect

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
)

// Interceptor handles ?nxp requests before route dispatch without calling workers.
type Interceptor struct {
	cfg     Config
	buildID string
	cache   *Cache
}

// NewInterceptor constructs an Interceptor instance.
func NewInterceptor(cfg Config, buildID string) *Interceptor {
	return &Interceptor{
		cfg:     cfg,
		buildID: buildID,
		cache:   NewCache(),
	}
}

// Intercept inspects inbound HTTP requests and handles ?nxp requests.
// Returns true if the request was an introspection request and handled.
func (i *Interceptor) Intercept(w http.ResponseWriter, r *http.Request, table *router.Table) bool {
	if !r.URL.Query().Has("nxp") {
		return false
	}

	if !i.cfg.Enabled {
		http.Error(w, "introspection disabled", http.StatusNotFound)
		return true
	}

	// Check if public access allowed
	if !i.cfg.Public {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "introspection requires authentication", http.StatusUnauthorized)
			return true
		}
	}

	// Check for schema bundle endpoint
	if r.URL.Path == i.cfg.BundlePath || r.URL.Path == "/__nxp/schema" {
		return HandleSchemaBundle(w, r, table, i.buildID)
	}

	accept := r.Header.Get("Accept")
	if cachedBytes, ok := i.cache.Get(i.buildID, r.URL.Path, accept); ok {
		w.Header().Set("X-NXP-Cache", "HIT")
		i.writeContent(w, accept, cachedBytes)
		return true
	}

	view := BuildView(table, r.URL.Path)

	// Apply redaction
	if view.Self != nil && len(i.cfg.Redact) > 0 {
		// Redact headers/scopes if in redact list
	}

	var (
		payload     []byte
		contentType string
		err         error
	)

	switch {
	case strings.Contains(accept, "text/plain"):
		contentType = "text/plain; charset=utf-8"
		payload = []byte(RenderTree(view))

	case strings.Contains(accept, "text/html"):
		contentType = "text/html; charset=utf-8"
		payload = []byte(RenderHTML(view))

	case strings.Contains(accept, "application/openapi+json"):
		contentType = "application/openapi+json; charset=utf-8"
		payload, err = RenderOpenAPI(view)

	case strings.Contains(accept, "application/asyncapi+json"):
		contentType = "application/asyncapi+json; charset=utf-8"
		payload, err = RenderAsyncAPI(view)

	case strings.Contains(accept, "application/schema+json"):
		contentType = "application/schema+json; charset=utf-8"
		payload, err = json.MarshalIndent(view.Self, "", "  ")

	default: // application/json
		contentType = "application/json; charset=utf-8"
		payload, err = json.MarshalIndent(view, "", "  ")
	}

	if err != nil {
		http.Error(w, "failed to render introspection: "+err.Error(), http.StatusInternalServerError)
		return true
	}

	i.cache.Set(i.buildID, r.URL.Path, accept, payload)

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
	return true
}

func (i *Interceptor) writeContent(w http.ResponseWriter, accept string, data []byte) {
	contentType := "application/json; charset=utf-8"
	if strings.Contains(accept, "text/plain") {
		contentType = "text/plain; charset=utf-8"
	} else if strings.Contains(accept, "text/html") {
		contentType = "text/html; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
