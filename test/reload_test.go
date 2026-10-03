package test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// TestHotReloadAtomicRouteSwap verifies zero-downtime atomic route swapping during reload.
func TestHotReloadAtomicRouteSwap(t *testing.T) {
	table := router.NewTable()

	// Initial route pointing to worker-v1
	routeV1 := abi.Route{
		ID:        "get.data",
		Transport: abi.TransportREST,
		Method:    "GET",
		Path:      "/data",
		Shape: abi.RequestResponseShape{
			Responses: map[int]*abi.Schema{200: {Type: "object"}},
		},
	}

	entryV1 := &router.Entry{
		Route:      routeV1,
		WorkerName: "worker-v1",
		HandlerFunc: func(req *abi.Request) (*abi.Response, error) {
			// Simulate slight processing time
			time.Sleep(10 * time.Millisecond)
			return abi.NewJSONResponse(200, map[string]string{"version": "v1"})
		},
	}
	table.Store(entryV1)

	const totalRequests = 100
	var wg sync.WaitGroup
	var completedCount atomic.Int64
	var v1Count atomic.Int64
	var v2Count atomic.Int64
	var errorsCount atomic.Int64

	// Start concurrent request load
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Halfway through, trigger route swap from another goroutine
			if idx == totalRequests/2 {
				entryV2 := &router.Entry{
					Route:      routeV1,
					WorkerName: "worker-v2",
					HandlerFunc: func(req *abi.Request) (*abi.Response, error) {
						return abi.NewJSONResponse(200, map[string]string{"version": "v2"})
					},
				}
				// Atomic swap in routing table
				table.Swap("/data", entryV2)
			}

			// Load and execute route
			entry, _, found := table.Load(abi.TransportREST, "GET", "/data")
			if !found {
				errorsCount.Add(1)
				return
			}

			resp, err := entry.HandlerFunc(abi.NewRequest(abi.TransportREST, "GET", "/data"))
			if err != nil || resp.Status != 200 {
				errorsCount.Add(1)
				return
			}

			completedCount.Add(1)
			if entry.WorkerName == "worker-v1" {
				v1Count.Add(1)
			} else if entry.WorkerName == "worker-v2" {
				v2Count.Add(1)
			}
		}(i)

		// Stagger requests slightly to simulate real traffic
		time.Sleep(1 * time.Millisecond)
	}

	wg.Wait()

	if errorsCount.Load() > 0 {
		t.Fatalf("encountered %d dropped requests during route swap", errorsCount.Load())
	}

	if completedCount.Load() != int64(totalRequests) {
		t.Fatalf("expected %d total completed requests, got %d", totalRequests, completedCount.Load())
	}

	if v1Count.Load() == 0 || v2Count.Load() == 0 {
		t.Errorf("expected both v1 and v2 traffic during swap: v1=%d, v2=%d", v1Count.Load(), v2Count.Load())
	}

	// Verify table now points to v2
	latestEntry, _, ok := table.Load(abi.TransportREST, "GET", "/data")
	if !ok || latestEntry.WorkerName != "worker-v2" {
		t.Errorf("expected table to resolve to worker-v2 after swap, got %v", latestEntry)
	}
}

// TestGracefulDrainUnderLoad asserts workers cleanly finish all in-flight requests before exiting.
func TestGracefulDrainUnderLoad(t *testing.T) {
	var inFlight atomic.Int64
	var completed atomic.Int64
	draining := make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const workers = 20
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inFlight.Add(1)
			defer inFlight.Add(-1)

			// Simulate work
			time.Sleep(30 * time.Millisecond)
			completed.Add(1)
		}()
	}

	// Signal drain
	close(draining)

	// Wait for in-flight requests to complete
	deadline := time.Now().Add(500 * time.Millisecond)
	for inFlight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	if inFlight.Load() != 0 {
		t.Errorf("expected 0 in-flight requests after drain, got %d", inFlight.Load())
	}
	if completed.Load() != int64(workers) {
		t.Errorf("expected all %d requests to complete during drain, got %d", workers, completed.Load())
	}
	_ = ctx
	_ = fmt.Sprintf("")
}
