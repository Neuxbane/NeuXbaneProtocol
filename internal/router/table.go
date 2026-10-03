// Package router answers: which handler and worker serve a given inbound request across transports?
package router

import (
	"strings"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// ChildKind indicates whether a child route is a leaf handler or an intermediate branch.
type ChildKind string

const (
	ChildKindLeaf   ChildKind = "leaf"
	ChildKindBranch ChildKind = "branch"
)

// Child represents a direct sub-route beneath a inspected path prefix.
type Child struct {
	Name     string    `json:"name"`
	Kind     ChildKind `json:"kind"`
	Dynamic  bool      `json:"dynamic,omitempty"`
	HasIndex bool      `json:"hasIndex,omitempty"`
	Methods  []string  `json:"methods,omitempty"`
	Auth     string    `json:"auth,omitempty"`
}

// Entry binds a compiled Route to its execution target.
type Entry struct {
	Route        abi.Route
	WorkerName   string
	WorkerSocket string
	HandlerID    abi.HandlerID
	HandlerFunc  func(*abi.Request) (*abi.Response, error)
	Metadata     map[string]string

	pattern *pathPattern
}

// MakeRouteKey formats the canonical index key for an exact route.
func MakeRouteKey(transport abi.Transport, method, path string) string {
	cleanPath := cleanRoutePath(path)
	return string(transport) + ":" + strings.ToUpper(method) + ":" + cleanPath
}

// Table maintains the active thread-safe registry of all routes across all transports.
type Table struct {
	mu      sync.RWMutex
	exact   map[string]*Entry
	dynamic []*Entry // parameterized routes
}

// NewTable constructs an empty Table.
func NewTable() *Table {
	return &Table{
		exact:   make(map[string]*Entry),
		dynamic: make([]*Entry, 0),
	}
}

// Store registers a route entry into the routing table.
func (t *Table) Store(entry *Entry) {
	if entry == nil {
		return
	}
	entry.Route.Path = cleanRoutePath(entry.Route.Path)
	pattern := parsePattern(entry.Route.Path)
	entry.pattern = pattern

	key := MakeRouteKey(entry.Route.Transport, entry.Route.Method, entry.Route.Path)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.exact[key] = entry

	if pattern.isDynamic {
		// Update or append to dynamic entries
		found := false
		for i, existing := range t.dynamic {
			if existing.Route.Transport == entry.Route.Transport &&
				strings.EqualFold(existing.Route.Method, entry.Route.Method) &&
				existing.Route.Path == entry.Route.Path {
				t.dynamic[i] = entry
				found = true
				break
			}
		}
		if !found {
			t.dynamic = append(t.dynamic, entry)
		}
	}
}

// Load looks up an Entry matching transport, method, and request path.
// It returns the entry, extracted path parameters, and true if found.
func (t *Table) Load(transport abi.Transport, method, path string) (*Entry, map[string]string, bool) {
	cleanPath := cleanRoutePath(path)
	key := MakeRouteKey(transport, method, cleanPath)

	t.mu.RLock()
	defer t.mu.RUnlock()

	// 1. Try exact match first
	if entry, ok := t.exact[key]; ok {
		return entry, nil, true
	}

	// 2. Try dynamic/parameterized patterns
	normMethod := strings.ToUpper(method)
	for _, entry := range t.dynamic {
		if entry.Route.Transport != transport {
			continue
		}
		if !strings.EqualFold(entry.Route.Method, normMethod) && entry.Route.Method != "*" {
			continue
		}
		if params, ok := entry.pattern.match(cleanPath); ok {
			return entry, params, true
		}
	}

	return nil, nil, false
}

// LoadKey directly retrieves an entry by exact route key.
func (t *Table) LoadKey(key string) (*Entry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entry, ok := t.exact[key]
	return entry, ok
}

// Self returns the route definition residing at the exact path (e.g. index handler).
func (t *Table) Self(path string) (*abi.Route, bool) {
	cleanPath := cleanRoutePath(path)
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, entry := range t.exact {
		if entry.Route.Path == cleanPath {
			cp := entry.Route
			return &cp, true
		}
	}
	return nil, false
}

// SelfEntries returns all entries for all methods at the exact path.
func (t *Table) SelfEntries(path string) []*Entry {
	cleanPath := cleanRoutePath(path)
	t.mu.RLock()
	defer t.mu.RUnlock()

	var matches []*Entry
	for _, entry := range t.exact {
		if entry.Route.Path == cleanPath {
			matches = append(matches, entry)
		}
	}
	return matches
}

// Children returns all immediate, one-level direct children under the specified prefix.
// It never returns 'index', collapses dynamic segments into {id}, and aggregates methods.
func (t *Table) Children(prefix string) []Child {
	cleanPrefix := cleanRoutePath(prefix)
	if cleanPrefix == "/" {
		cleanPrefix = ""
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	type childAgg struct {
		child    Child
		subPaths map[string]bool
	}
	aggMap := make(map[string]*childAgg)

	for _, entry := range t.exact {
		routePath := entry.Route.Path
		if routePath == cleanPrefix || routePath == cleanPrefix+"/" {
			continue
		}

		if !strings.HasPrefix(routePath, cleanPrefix+"/") {
			continue
		}

		// Remainder after prefix + "/"
		remainder := strings.TrimPrefix(routePath, cleanPrefix+"/")
		segments := strings.Split(remainder, "/")
		if len(segments) == 0 || segments[0] == "" {
			continue
		}

		childSegment := segments[0]
		// Collapse any "[id]" to "{id}"
		if strings.HasPrefix(childSegment, "[") && strings.HasSuffix(childSegment, "]") {
			childSegment = "{" + childSegment[1:len(childSegment)-1] + "}"
		}

		isDynamic := strings.HasPrefix(childSegment, "{") && strings.HasSuffix(childSegment, "}")
		isDirectLeaf := len(segments) == 1

		agg, exists := aggMap[childSegment]
		if !exists {
			kind := ChildKindLeaf
			if !isDirectLeaf {
				kind = ChildKindBranch
			}
			agg = &childAgg{
				child: Child{
					Name:    childSegment,
					Kind:    kind,
					Dynamic: isDynamic,
					Methods: make([]string, 0),
					Auth:    entry.Route.Auth,
				},
				subPaths: make(map[string]bool),
			}
			aggMap[childSegment] = agg
		}

		if !isDirectLeaf {
			agg.child.Kind = ChildKindBranch
		}

		if isDirectLeaf {
			// Add method if not present
			methodPresent := false
			for _, m := range agg.child.Methods {
				if m == entry.Route.Method {
					methodPresent = true
					break
				}
			}
			if !methodPresent && entry.Route.Method != "" {
				agg.child.Methods = append(agg.child.Methods, entry.Route.Method)
			}
		}

		agg.subPaths[routePath] = true
	}

	// For branches, check if they have an index handler
	for _, agg := range aggMap {
		if agg.child.Kind == ChildKindBranch {
			branchPath := cleanPrefix + "/" + agg.child.Name
			for p := range agg.subPaths {
				if p == branchPath {
					agg.child.HasIndex = true
					break
				}
			}
		}
	}

	result := make([]Child, 0, len(aggMap))
	for _, agg := range aggMap {
		result = append(result, agg.child)
	}
	return result
}

// AllEntries returns a snapshot of all registered entries.
func (t *Table) AllEntries() []*Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entries := make([]*Entry, 0, len(t.exact))
	for _, e := range t.exact {
		entries = append(entries, e)
	}
	return entries
}

// Count returns the total number of exact routes stored.
func (t *Table) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.exact)
}

func cleanRoutePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Normalize trailing slash except for root "/"
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}
