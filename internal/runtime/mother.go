package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/codegen"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/config"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/scaffold"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/telemetry"
)

// Mother is the top-level supervisor process orchestrating routes, workers, transports, and hot-reload.
type Mother struct {
	cfg            *config.Config
	table          *router.Table
	registry       *Registry
	supervisor     *Supervisor
	watcher        *Watcher
	currentBuildID string
	onReloadHooks  []func(buildID string)
	mu             sync.Mutex
	cancelFunc     context.CancelFunc
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

// OnWorkerReload registers a hook invoked when a new worker build is activated.
func (m *Mother) OnWorkerReload(fn func(buildID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onReloadHooks = append(m.onReloadHooks, fn)
}

// Start begins file watching and starts background lifecycle monitoring.
func (m *Mother) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	workDir := filepath.Dir(m.cfg.DefineDir)
	if workDir == "" {
		workDir = "."
	}

	// 1. Scaffold workspace (define/, local abi/, go.mod, and README.md) if missing
	if err := scaffold.EnsureAll(workDir, m.cfg.DefineDir); err != nil {
		telemetry.Logger().Error("failed to ensure developer workspace", "err", err)
	}

	// 2. Perform initial worker codegen, build, and spawn
	if err := m.RebuildAndSwapWorker(ctx); err != nil {
		telemetry.Logger().Warn("initial worker build/spawn notice", "err", err)
	}

	// 3. Start file watcher if in dev mode
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

	telemetry.LogService("mother", "runtime started on env=%s (sock_dir: %s)", m.cfg.Env, m.cfg.SocketDir)
	return nil
}

// RebuildAndSwapWorker compiles the worker from defineDir and atomically swaps routes.
func (m *Mother) RebuildAndSwapWorker(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	workDir := filepath.Dir(m.cfg.DefineDir)
	if workDir == "" {
		workDir = "."
	}

	modulePath := scaffold.DetectModulePath(m.cfg.DefineDir)
	workerDir := filepath.Join(workDir, ".nxp", "worker")
	buildIDFile := filepath.Join(workDir, ".nxp-build-id")

	buildID, err := codegen.GenerateWorkerProject(m.cfg.DefineDir, modulePath, workerDir, buildIDFile)
	if err != nil {
		return fmt.Errorf("codegen error: %w", err)
	}

	if buildID == m.currentBuildID {
		telemetry.LogService("mother", "build ID unchanged (%s), skipping reload", buildID)
		return nil
	}

	if err := os.MkdirAll(m.cfg.SocketDir, 0755); err != nil {
		return fmt.Errorf("mkdir socket dir: %w", err)
	}

	binPath := filepath.Join(m.cfg.SocketDir, fmt.Sprintf("nxp-worker-%s", buildID))
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./.nxp/worker")
	buildCmd.Dir = workDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("go build worker error: %w", err)
	}

	_, err = m.supervisor.SpawnWorker(ctx, "nxp-worker", buildID, binPath)
	if err != nil {
		return fmt.Errorf("spawn worker error: %w", err)
	}

	m.currentBuildID = buildID
	for _, hook := range m.onReloadHooks {
		hook(buildID)
	}
	telemetry.LogService("mother", "worker live and route swap completed (build: %s)", buildID)
	return nil
}

// TriggerReload triggers rebuilding and hot swapping workers when define/ changes.
func (m *Mother) TriggerReload(ctx context.Context) {
	telemetry.LogService("mother", "triggering hot reload of workers from define/ changes")
	if err := m.RebuildAndSwapWorker(ctx); err != nil {
		telemetry.LogService("mother", "ERROR worker reload failed: %v", err)
	}
}

// Dispatch forwards an inbound request through the supervisor to the worker.
func (m *Mother) Dispatch(ctx context.Context, req *abi.Request) (*abi.Response, error) {
	return m.supervisor.Dispatch(ctx, req)
}

// Shutdown drains all workers and terminates background tasks.
func (m *Mother) Shutdown(ctx context.Context) error {
	telemetry.LogService("mother", "shutting down mother process, draining workers...")
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.watcher != nil {
		_ = m.watcher.Close()
	}

	// Drain all workers
	err := m.supervisor.DrainAll(ctx)

	// Remove only the worker binary this instance built. Never RemoveAll the
	// socket directory: it may be shared with other running nxp servers, and
	// wiping it would delete their worker binaries and sockets.
	if m.currentBuildID != "" {
		binPath := filepath.Join(m.cfg.SocketDir, fmt.Sprintf("nxp-worker-%s", m.currentBuildID))
		_ = os.Remove(binPath)
	}
	telemetry.LogService("mother", "mother process shutdown complete")
	return err
}
