package introspect

import (
	"sync"
)

type cacheKey struct {
	buildID string
	path    string
	accept  string
}

// Cache caches formatted responses keyed by buildId, path, and accept header.
type Cache struct {
	mu    sync.RWMutex
	items map[cacheKey][]byte
}

// NewCache constructs an initialized Cache.
func NewCache() *Cache {
	return &Cache{
		items: make(map[cacheKey][]byte),
	}
}

// Get returns the cached byte response.
func (c *Cache) Get(buildID, path, accept string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, ok := c.items[cacheKey{buildID, path, accept}]
	return b, ok
}

// Set stores the byte response in the cache.
func (c *Cache) Set(buildID, path, accept string, val []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cacheKey{buildID, path, accept}] = val
}

// Clear purges all cached views.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[cacheKey][]byte)
}
