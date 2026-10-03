package router

import (
	"strings"
)

// Swap atomically replaces the entry at the path/method/transport with newEntry.
// It returns the previously registered entry, or nil if none was present.
func (t *Table) Swap(path string, newEntry *Entry) *Entry {
	if newEntry == nil {
		return nil
	}
	newEntry.Route.Path = cleanRoutePath(path)
	pattern := parsePattern(newEntry.Route.Path)
	newEntry.pattern = pattern

	key := MakeRouteKey(newEntry.Route.Transport, newEntry.Route.Method, newEntry.Route.Path)

	t.mu.Lock()
	defer t.mu.Unlock()

	old := t.exact[key]
	t.exact[key] = newEntry

	// Update dynamic entries list if applicable
	if pattern.isDynamic {
		found := false
		for i, existing := range t.dynamic {
			if existing.Route.Transport == newEntry.Route.Transport &&
				strings.EqualFold(existing.Route.Method, newEntry.Route.Method) &&
				existing.Route.Path == newEntry.Route.Path {
				t.dynamic[i] = newEntry
				found = true
				break
			}
		}
		if !found {
			t.dynamic = append(t.dynamic, newEntry)
		}
	} else if old != nil && old.pattern != nil && old.pattern.isDynamic {
		// If old was dynamic and new is not, remove from dynamic
		filtered := make([]*Entry, 0, len(t.dynamic))
		for _, e := range t.dynamic {
			if e != old {
				filtered = append(filtered, e)
			}
		}
		t.dynamic = filtered
	}

	return old
}

// SwapWorker atomically replaces all entries associated with a worker with newEntries.
// It returns all removed entries so that in-flight requests can be drained.
func (t *Table) SwapWorker(workerName string, newEntries []*Entry) []*Entry {
	t.mu.Lock()
	defer t.mu.Unlock()

	var removed []*Entry

	// 1. Remove all existing entries for this worker
	for k, entry := range t.exact {
		if entry.WorkerName == workerName {
			removed = append(removed, entry)
			delete(t.exact, k)
		}
	}

	// Filter dynamic entries
	filteredDynamic := make([]*Entry, 0, len(t.dynamic))
	for _, entry := range t.dynamic {
		if entry.WorkerName != workerName {
			filteredDynamic = append(filteredDynamic, entry)
		}
	}
	t.dynamic = filteredDynamic

	// 2. Insert all new entries
	for _, newEntry := range newEntries {
		if newEntry == nil {
			continue
		}
		newEntry.WorkerName = workerName
		newEntry.Route.Path = cleanRoutePath(newEntry.Route.Path)
		pattern := parsePattern(newEntry.Route.Path)
		newEntry.pattern = pattern

		key := MakeRouteKey(newEntry.Route.Transport, newEntry.Route.Method, newEntry.Route.Path)
		t.exact[key] = newEntry

		if pattern.isDynamic {
			t.dynamic = append(t.dynamic, newEntry)
		}
	}

	return removed
}

// DeleteWorker removes all routes associated with a worker and returns them.
func (t *Table) DeleteWorker(workerName string) []*Entry {
	return t.SwapWorker(workerName, nil)
}
