// Package runtime answers: how does the mother process orchestrate workers, route tables, file watching, and process lifecycles?
package runtime

import (
	"sync"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/ipc"
)

// WorkerInstance represents a live worker OS process connected to the mother.
type WorkerInstance struct {
	Name       string
	BuildID    string
	PID        int
	SocketPath string
	Client     *ipc.Client
	StartedAt  time.Time
	Routes     []abi.Route
	Draining   bool
}

// Registry maintains active worker instances and the reverse index of routes to workers.
type Registry struct {
	mu           sync.RWMutex
	workers      map[string]*WorkerInstance
	routeReverse map[abi.HandlerID]string // HandlerID -> workerName
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		workers:      make(map[string]*WorkerInstance),
		routeReverse: make(map[abi.HandlerID]string),
	}
}

// Register adds or updates a worker instance and updates reverse route mapping.
func (r *Registry) Register(inst *WorkerInstance) {
	if inst == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.workers[inst.Name] = inst
	for _, route := range inst.Routes {
		r.routeReverse[route.ID] = inst.Name
	}
}

// Unregister removes a worker instance and purges its reverse route mapping.
func (r *Registry) Unregister(workerName string) *WorkerInstance {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, ok := r.workers[workerName]
	if !ok {
		return nil
	}

	delete(r.workers, workerName)
	for _, route := range inst.Routes {
		if r.routeReverse[route.ID] == workerName {
			delete(r.routeReverse, route.ID)
		}
	}
	return inst
}

// Get returns the WorkerInstance for a given worker name.
func (r *Registry) Get(workerName string) (*WorkerInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.workers[workerName]
	return inst, ok
}

// FindByHandler locates the WorkerInstance owning a specific HandlerID.
func (r *Registry) FindByHandler(id abi.HandlerID) (*WorkerInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workerName, ok := r.routeReverse[id]
	if !ok {
		return nil, false
	}
	inst, ok := r.workers[workerName]
	return inst, ok
}

// All returns a slice of all active worker instances.
func (r *Registry) All() []*WorkerInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*WorkerInstance, 0, len(r.workers))
	for _, inst := range r.workers {
		list = append(list, inst)
	}
	return list
}
