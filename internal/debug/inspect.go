// Package debug answers: what internal inspection endpoints, pprof handlers, and reload triggers are available for operators?
package debug

import (
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/runtime"
)

// Server provides the HTTP mux for debugging, pprof, inspection, and force reload.
type Server struct {
	mother *runtime.Mother
	mux    *http.ServeMux
}

// NewServer constructs an initialized debug Server.
func NewServer(mother *runtime.Mother) *Server {
	s := &Server{
		mother: mother,
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the http.Handler for the debug server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	// Standard pprof
	s.mux.HandleFunc("/debug/pprof/", pprof.Index)
	s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Inspection endpoints
	s.mux.HandleFunc("/debug/inspect/workers", s.handleWorkers)
	s.mux.HandleFunc("/debug/inspect/routes", s.handleRoutes)
	s.mux.HandleFunc("/debug/inspect/contracts", s.handleContracts)

	// Reload endpoint
	s.mux.HandleFunc("/debug/reload", s.handleReload)
}

func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	workers := s.mother.Registry().All()

	type workerView struct {
		Name       string    `json:"name"`
		BuildID    string    `json:"build_id"`
		PID        int       `json:"pid"`
		StartedAt  time.Time `json:"started_at"`
		RouteCount int       `json:"route_count"`
		Draining   bool      `json:"draining"`
	}

	views := make([]workerView, 0, len(workers))
	for _, inst := range workers {
		views = append(views, workerView{
			Name:       inst.Name,
			BuildID:    inst.BuildID,
			PID:        inst.PID,
			StartedAt:  inst.StartedAt,
			RouteCount: len(inst.Routes),
			Draining:   inst.Draining,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(views)
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	entries := s.mother.Table().AllEntries()

	transportFilter := r.URL.Query().Get("transport")
	prefixFilter := r.URL.Query().Get("prefix")

	type routeView struct {
		ID        string `json:"id"`
		Transport string `json:"transport"`
		Method    string `json:"method"`
		Path      string `json:"path"`
		Worker    string `json:"worker"`
	}

	views := make([]routeView, 0, len(entries))
	for _, e := range entries {
		if transportFilter != "" && string(e.Route.Transport) != transportFilter {
			continue
		}
		if prefixFilter != "" && !strings.HasPrefix(e.Route.Path, prefixFilter) {
			continue
		}

		views = append(views, routeView{
			ID:        string(e.Route.ID),
			Transport: string(e.Route.Transport),
			Method:    e.Route.Method,
			Path:      e.Route.Path,
			Worker:    e.WorkerName,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(views)
}

func (s *Server) handleContracts(w http.ResponseWriter, r *http.Request) {
	entries := s.mother.Table().AllEntries()
	contracts := make(map[string]any)

	for _, e := range entries {
		contracts[string(e.Route.ID)] = e.Route.Shape
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contracts)
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	s.mother.TriggerReload(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "reload_triggered",
		"message": "worker hot reload initiated",
	})
}
