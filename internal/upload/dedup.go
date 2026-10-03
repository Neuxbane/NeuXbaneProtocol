package upload

import (
	"sync"
)

// DedupIndex stores content-addressable SHA256 hashes mapped to existing file IDs.
type DedupIndex struct {
	mu    sync.RWMutex
	items map[string]string // sha256 -> existingID
}

// GlobalDedup provides the process-wide deduplication index.
var GlobalDedup = NewDedupIndex()

// NewDedupIndex constructs an initialized DedupIndex.
func NewDedupIndex() *DedupIndex {
	return &DedupIndex{
		items: make(map[string]string),
	}
}

// Check queries whether a SHA256 has already been stored.
func (d *DedupIndex) Check(sha256 string) (existingID string, hit bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	id, ok := d.items[sha256]
	return id, ok
}

// Register records a new SHA256 mapping.
func (d *DedupIndex) Register(sha256, id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items[sha256] = id
}
