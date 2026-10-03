package debug_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/debug"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/runtime"
)

func TestDebugEndpoints(t *testing.T) {
	mother, err := runtime.NewMother(config.DefaultConfig())
	if err != nil {
		t.Fatalf("NewMother failed: %v", err)
	}

	debugSrv := debug.NewServer(mother)
	handler := debugSrv.Handler()

	// 1. /debug/inspect/workers
	req := httptest.NewRequest("GET", "/debug/inspect/workers", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("workers inspect status = %d, want 200", rec.Code)
	}

	// 2. /debug/inspect/routes
	reqRoutes := httptest.NewRequest("GET", "/debug/inspect/routes", nil)
	recRoutes := httptest.NewRecorder()
	handler.ServeHTTP(recRoutes, reqRoutes)
	if recRoutes.Code != http.StatusOK {
		t.Errorf("routes inspect status = %d, want 200", recRoutes.Code)
	}

	// 3. /debug/inspect/contracts
	reqContracts := httptest.NewRequest("GET", "/debug/inspect/contracts", nil)
	recContracts := httptest.NewRecorder()
	handler.ServeHTTP(recContracts, reqContracts)
	if recContracts.Code != http.StatusOK {
		t.Errorf("contracts inspect status = %d, want 200", recContracts.Code)
	}

	// 4. /debug/reload
	reqReload := httptest.NewRequest("POST", "/debug/reload", nil)
	recReload := httptest.NewRecorder()
	handler.ServeHTTP(recReload, reqReload)
	if recReload.Code != http.StatusOK {
		t.Errorf("reload status = %d, want 200", recReload.Code)
	}
}
