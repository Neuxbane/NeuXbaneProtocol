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

// SetBuildID updates the active build ID and invalidates cached views.
func (i *Interceptor) SetBuildID(buildID string) {
	i.buildID = buildID
	if i.cache != nil {
		i.cache.Clear()
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

	nxpParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("nxp")))
	accept := r.Header.Get("Accept")

	format := "json" // default is JSON!

	if nxpParam == "html" {
		format = "html"
	} else if nxpParam == "json" {
		format = "json"
	} else if nxpParam == "tree" || nxpParam == "text" {
		format = "text"
	} else if nxpParam == "openapi" {
		format = "openapi"
	} else if nxpParam == "asyncapi" {
		format = "asyncapi"
	} else if nxpParam == "schema" {
		format = "schema"
	} else {
		// nxpParam was empty or generic (e.g. ?nxp)
		if strings.Contains(accept, "text/plain") {
			format = "text"
		} else if strings.Contains(accept, "application/openapi+json") {
			format = "openapi"
		} else if strings.Contains(accept, "application/asyncapi+json") {
			format = "asyncapi"
		} else if strings.Contains(accept, "application/schema+json") {
			format = "schema"
		} else if accept == "text/html" {
			format = "html"
		} else {
			format = "json"
		}
	}

	cacheKey := format
	if cachedBytes, ok := i.cache.Get(i.buildID, r.URL.Path, cacheKey); ok {
		w.Header().Set("X-NXP-Cache", "HIT")
		i.writeContent(w, format, cachedBytes)
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

	switch format {
	case "text":
		contentType = "text/plain; charset=utf-8"
		payload = []byte(RenderTree(view))

	case "html":
		contentType = "text/html; charset=utf-8"
		payload = []byte(RenderHTML(view))

	case "openapi":
		contentType = "application/openapi+json; charset=utf-8"
		payload, err = RenderOpenAPI(view)

	case "asyncapi":
		contentType = "application/asyncapi+json; charset=utf-8"
		payload, err = RenderAsyncAPI(view)

	case "schema":
		contentType = "application/schema+json; charset=utf-8"
		payload, err = json.MarshalIndent(view.Self, "", "  ")

	default: // "json"
		contentType = "application/json; charset=utf-8"
		payload, err = json.MarshalIndent(view, "", "  ")
	}

	if err != nil {
		http.Error(w, "failed to render introspection: "+err.Error(), http.StatusInternalServerError)
		return true
	}

	i.cache.Set(i.buildID, r.URL.Path, cacheKey, payload)

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
	return true
}

func (i *Interceptor) writeContent(w http.ResponseWriter, format string, data []byte) {
	contentType := "application/json; charset=utf-8"
	switch format {
	case "text":
		contentType = "text/plain; charset=utf-8"
	case "html":
		contentType = "text/html; charset=utf-8"
	case "openapi":
		contentType = "application/openapi+json; charset=utf-8"
	case "asyncapi":
		contentType = "application/asyncapi+json; charset=utf-8"
	case "schema":
		contentType = "application/schema+json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
