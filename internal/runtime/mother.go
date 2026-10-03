package runtime

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/telemetry"
)

// Mother is the top-level supervisor process orchestrating routes, workers, transports, and hot-reload.
type Mother struct {
	cfg        *config.Config
	table      *router.Table
	registry   *Registry
	supervisor *Supervisor
	watcher    *Watcher
	mu         sync.Mutex
	cancelFunc context.CancelFunc
}

// NewMother constructs an initialized Mother process instance.
func NewMother(cfg *config.Config) (*Mother, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	table := router.NewTable()
	registry := NewRegistry()
	supervisor := NewSupervisor(table, registry, cfg.SocketDir)

	m := &Mother{
		cfg:        cfg,
		table:      table,
		registry:   registry,
		supervisor: supervisor,
	}

	return m, nil
}

// Table returns the active routing table.
func (m *Mother) Table() *router.Table {
	return m.table
}

// Registry returns the active worker registry.
func (m *Mother) Registry() *Registry {
	return m.registry
}

// Supervisor returns the worker supervisor.
func (m *Mother) Supervisor() *Supervisor {
	return m.supervisor
}

// Start begins file watching and starts background lifecycle monitoring.
func (m *Mother) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	// Start file watcher if in dev mode
	if m.cfg.Env == config.EnvDev {
		watcher, err := NewWatcher(m.cfg.DefineDir, m.cfg.Debounce, func(files []string) {
			m.TriggerReload(context.Background())
		})
		if err != nil {
			telemetry.Logger().Warn("failed to initialize define/ watcher", "err", err)
		} else {
			m.watcher = watcher
			_ = m.watcher.Start(ctx)
		}
	}

	telemetry.Logger().Info("mother runtime started", "env", m.cfg.Env, "sock_dir", m.cfg.SocketDir)
	return nil
}

// TriggerReload triggers rebuilding and hot swapping workers when define/ changes.
func (m *Mother) TriggerReload(ctx context.Context) {
	telemetry.Logger().Info("triggering hot reload of workers")
	// Rebuild and spawn updated worker
	buildID := fmt.Sprintf("bld-%d", time.Now().UnixNano())
	_, err := m.supervisor.BuildAndSpawnWorker(ctx, "nxp-worker", buildID, "./cmd/nxp-worker", "")
	if err != nil {
		telemetry.Logger().Error("worker reload failed", "err", err)
	} else {
		telemetry.Logger().Info("worker hot reload succeeded", "build_id", buildID)
	}
}

// Dispatch forwards an inbound request through the supervisor to the worker.
func (m *Mother) Dispatch(ctx context.Context, req *abi.Request) (*abi.Response, error) {
	return m.supervisor.Dispatch(ctx, req)
}

// Shutdown drains all workers and terminates background tasks.
func (m *Mother) Shutdown(ctx context.Context) error {
	telemetry.Logger().Info("shutting down mother process, draining workers...")
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.watcher != nil {
		_ = m.watcher.Close()
	}

	// Drain all workers
	err := m.supervisor.DrainAll(ctx)
	_ = os.RemoveAll(m.cfg.SocketDir)
	telemetry.Logger().Info("mother process shutdown complete")
	return err
}
