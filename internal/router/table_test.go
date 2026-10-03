package router_test

import (
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
)

func TestStoreAndLoadExact(t *testing.T) {
	tbl := router.NewTable()

	entry := &router.Entry{
		Route: abi.Route{
			ID:        "auth.login",
			Transport: abi.TransportREST,
			Method:    "POST",
			Path:      "/auth/login",
			Auth:      "public",
		},
		WorkerName: "worker-auth",
		HandlerID:  "auth.login",
	}

	tbl.Store(entry)

	// Successful lookup
	loaded, params, ok := tbl.Load(abi.TransportREST, "POST", "/auth/login")
	if !ok {
		t.Fatalf("expected route to be found")
	}
	if loaded.HandlerID != "auth.login" {
		t.Errorf("expected auth.login, got %s", loaded.HandlerID)
	}
	if len(params) != 0 {
		t.Errorf("expected no params for exact match, got %v", params)
	}

	// Method mismatch
	_, _, ok = tbl.Load(abi.TransportREST, "GET", "/auth/login")
	if ok {
		t.Errorf("expected GET to not match POST route")
	}

	// Transport mismatch
	_, _, ok = tbl.Load(abi.TransportWebSocket, "POST", "/auth/login")
	if ok {
		t.Errorf("expected websocket to not match REST route")
	}
}

func TestParameterizedRouting(t *testing.T) {
	tbl := router.NewTable()

	entry := &router.Entry{
		Route: abi.Route{
			ID:        "users.get",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/users/{id}",
		},
		WorkerName: "worker-users",
	}

	tbl.Store(entry)

	loaded, params, ok := tbl.Load(abi.TransportREST, "GET", "/users/usr_abc123")
	if !ok {
		t.Fatalf("expected parameterized route to match")
	}
	if loaded.Route.ID != "users.get" {
		t.Errorf("expected users.get, got %s", loaded.Route.ID)
	}
	if params["id"] != "usr_abc123" {
		t.Errorf("expected param id=usr_abc123, got %q", params["id"])
	}
}

func TestAtomicSwap(t *testing.T) {
	tbl := router.NewTable()

	entry1 := &router.Entry{
		Route: abi.Route{
			ID:        "item.get.v1",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/items/{id}",
		},
		WorkerName: "worker-v1",
	}
	tbl.Store(entry1)

	entry2 := &router.Entry{
		Route: abi.Route{
			ID:        "item.get.v2",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/items/{id}",
		},
		WorkerName: "worker-v2",
	}

	old := tbl.Swap("/items/{id}", entry2)
	if old == nil || old.WorkerName != "worker-v1" {
		t.Errorf("expected old entry to be worker-v1, got %v", old)
	}

	current, _, ok := tbl.Load(abi.TransportREST, "GET", "/items/99")
	if !ok || current.WorkerName != "worker-v2" {
		t.Errorf("expected swapped entry worker-v2, got %v", current)
	}
}

func TestSwapWorkerRoutes(t *testing.T) {
	tbl := router.NewTable()

	// Worker 1 routes
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "w1.r1",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/w1/a",
		},
		WorkerName: "worker-1",
	})
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "w1.r2",
			Transport: abi.TransportREST,
			Method:    "POST",
			Path:      "/w1/b",
		},
		WorkerName: "worker-1",
	})

	// Swap Worker 1 with new route
	newEntries := []*router.Entry{
		{
			Route: abi.Route{
				ID:        "w1.r3",
				Transport: abi.TransportREST,
				Method:    "GET",
				Path:      "/w1/c",
			},
		},
	}

	removed := tbl.SwapWorker("worker-1", newEntries)
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed entries, got %d", len(removed))
	}

	// Verify old routes gone
	_, _, ok1 := tbl.Load(abi.TransportREST, "GET", "/w1/a")
	_, _, ok2 := tbl.Load(abi.TransportREST, "POST", "/w1/b")
	if ok1 || ok2 {
		t.Errorf("expected old routes to be removed")
	}

	// Verify new route loaded
	loaded, _, ok := tbl.Load(abi.TransportREST, "GET", "/w1/c")
	if !ok || loaded.WorkerName != "worker-1" {
		t.Errorf("expected new route /w1/c for worker-1")
	}
}

func TestSelfAndChildren(t *testing.T) {
	tbl := router.NewTable()

	// /auth (index handler)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.index",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth",
			Auth:      "required",
		},
		WorkerName: "worker-auth",
	})

	// /auth/profile (GET and PATCH)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.profile.get",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth/profile",
			Auth:      "required",
		},
		WorkerName: "worker-auth",
	})
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.profile.patch",
			Transport: abi.TransportREST,
			Method:    "PATCH",
			Path:      "/auth/profile",
			Auth:      "required",
		},
		WorkerName: "worker-auth",
	})

	// /auth/{id}/tokens (branch)
	tbl.Store(&router.Entry{
		Route: abi.Route{
			ID:        "auth.tokens.get",
			Transport: abi.TransportREST,
			Method:    "GET",
			Path:      "/auth/{id}/tokens",
		},
		WorkerName: "worker-auth",
	})

	// 1. Test Self
	self, ok := tbl.Self("/auth")
	if !ok {
		t.Fatalf("expected self route at /auth")
	}
	if self.ID != "auth.index" {
		t.Errorf("expected auth.index, got %s", self.ID)
	}

	// 2. Test Children of /auth
	children := tbl.Children("/auth")
	if len(children) != 2 {
		t.Fatalf("expected 2 direct children, got %d: %+v", len(children), children)
	}

	childMap := make(map[string]router.Child)
	for _, c := range children {
		childMap[c.Name] = c
	}

	// Check leaf child 'profile'
	profile, exists := childMap["profile"]
	if !exists {
		t.Fatalf("child 'profile' not found in children: %+v", children)
	}
	if profile.Kind != router.ChildKindLeaf {
		t.Errorf("expected profile to be leaf, got %s", profile.Kind)
	}
	if len(profile.Methods) != 2 {
		t.Errorf("expected 2 methods (GET, PATCH) for profile, got %v", profile.Methods)
	}

	// Check branch child '{id}'
	idChild, exists := childMap["{id}"]
	if !exists {
		t.Fatalf("child '{id}' not found in children: %+v", children)
	}
	if idChild.Kind != router.ChildKindBranch {
		t.Errorf("expected {id} to be branch, got %s", idChild.Kind)
	}
	if !idChild.Dynamic {
		t.Errorf("expected {id} to be dynamic")
	}
}
