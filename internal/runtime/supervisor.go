package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/ipc"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/router"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/telemetry"
)

// Supervisor builds, spawns, manages, and routes requests to worker processes.
type Supervisor struct {
	table     *router.Table
	registry  *Registry
	socketDir string
	mu        sync.Mutex
	draining  sync.WaitGroup
}

// NewSupervisor constructs a Supervisor instance.
func NewSupervisor(table *router.Table, registry *Registry, socketDir string) *Supervisor {
	return &Supervisor{
		table:     table,
		registry:  registry,
		socketDir: socketDir,
	}
}

// BuildAndSpawnWorker builds the worker binary and spawns it.
func (s *Supervisor) BuildAndSpawnWorker(ctx context.Context, workerName, buildID, sourceDir, binPath string) (*WorkerInstance, error) {
	// If sourceDir is specified and binPath doesn't exist or needs build
	if sourceDir != "" {
		if binPath == "" {
			binPath = filepath.Join(s.socketDir, fmt.Sprintf("nxp-worker-%s-%s", workerName, buildID))
		}
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, sourceDir)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return nil, fmt.Errorf("build worker %s: %w", workerName, err)
		}
	}

	return s.SpawnWorker(ctx, workerName, buildID, binPath)
}

// SpawnWorker spawns an already compiled worker binary, connects via IPC, performs handshake, and swaps routes.
func (s *Supervisor) SpawnWorker(ctx context.Context, workerName, buildID, binPath string) (*WorkerInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.socketDir, 0755); err != nil {
		return nil, fmt.Errorf("create socket dir: %w", err)
	}

	sockPath := filepath.Join(s.socketDir, fmt.Sprintf("%s-%s-%d.sock", workerName, buildID, time.Now().UnixNano()))
	// Ensure old socket file removed
	_ = os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen worker socket %s: %w", sockPath, err)
	}
	defer listener.Close()

	// Spawn the worker process
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("WORKER_SOCK=%s", sockPath),
		fmt.Sprintf("WORKER_NAME=%s", workerName),
		fmt.Sprintf("BUILD_ID=%s", buildID),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = os.Remove(sockPath)
		return nil, fmt.Errorf("start worker process: %w", err)
	}

	// Set accept timeout
	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accChan := make(chan acceptResult, 1)
	go func() {
		conn, err := listener.Accept()
		accChan <- acceptResult{conn: conn, err: err}
	}()

	var conn net.Conn
	select {
	case res := <-accChan:
		if res.err != nil {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("accept worker connection: %w", res.err)
		}
		conn = res.conn
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("timed out waiting for worker %s to connect to socket", workerName)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return nil, ctx.Err()
	}

	// 1. Read Hello frame
	hello, err := ipc.ReadHello(conn)
	if err != nil {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("read hello from worker %s: %w", workerName, err)
	}

	// Verify ABI compatibility
	if hello.ABIVersion != abi.ABIVersion {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		return nil, errors.New(errors.CodeContractWorkerUnavailable,
			fmt.Sprintf("worker ABI %s incompatible with mother ABI %s", hello.ABIVersion, abi.ABIVersion), 503)
	}

	// 2. Prepare client and worker instance
	client := ipc.NewClient(conn)
	inst := &WorkerInstance{
		Name:       workerName,
		BuildID:    buildID,
		PID:        cmd.Process.Pid,
		SocketPath: sockPath,
		Client:     client,
		StartedAt:  time.Now(),
		Routes:     hello.Routes,
	}

	// 3. Atomically swap routes into router table
	var newEntries []*router.Entry
	for _, r := range hello.Routes {
		rCopy := r
		newEntries = append(newEntries, &router.Entry{
			Route:        rCopy,
			WorkerName:   workerName,
			WorkerSocket: sockPath,
			HandlerID:    r.ID,
		})
	}

	// Swap in new entries and get removed ones to drain
	oldEntries := s.table.SwapWorker(workerName, newEntries)

	// 4. Update Registry
	oldWorker := s.registry.Unregister(workerName)
	s.registry.Register(inst)
	telemetry.GlobalMetrics.ActiveWorkers.Add(1)

	// 5. Send Ready frame to new worker
	ready := &abi.Ready{
		ABIVersion: abi.ABIVersion,
		Status:     "ready",
		Message:    "route swap complete",
	}
	if err := ipc.SendReady(conn, ready); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("send ready to worker: %w", err)
	}

	// 6. Gracefully drain old worker in background if present
	if oldWorker != nil {
		s.draining.Add(1)
		go func(old *WorkerInstance, routes []*router.Entry) {
			defer s.draining.Done()
			s.drainOldWorker(old, routes)
		}(oldWorker, oldEntries)
	}

	return inst, nil
}

func (s *Supervisor) drainOldWorker(old *WorkerInstance, routes []*router.Entry) {
	if old == nil || old.Client == nil {
		return
	}
	old.Draining = true
	telemetry.GlobalMetrics.WorkerReloads.Add(1)

	// Send drain frame to old worker
	drainFrame := ipc.NewFrame(ipc.FrameTypeDrain, "drain", nil)
	_ = old.Client.Send(drainFrame)

	// Wait up to 5 seconds for in-flight requests to complete
	time.Sleep(5 * time.Second)

	_ = old.Client.Close()
	if old.SocketPath != "" {
		_ = os.Remove(old.SocketPath)
	}
	telemetry.GlobalMetrics.ActiveWorkers.Add(-1)
}

// Dispatch forwards an incoming request to the appropriate worker process.
func (s *Supervisor) Dispatch(ctx context.Context, req *abi.Request) (*abi.Response, error) {
	// 1. Look up route in routing table
	entry, params, ok := s.table.Load(req.Transport, req.Method, req.Path)
	if !ok {
		return abi.NewErrorResponse(errors.ErrNotFound), errors.ErrNotFound
	}

	req.RoutePath = entry.Route.Path
	if len(params) > 0 {
		if req.Params == nil {
			req.Params = make(map[string]string)
		}
		for k, v := range params {
			req.Params[k] = v
		}
	}

	// 2. Locate worker instance
	inst, ok := s.registry.Get(entry.WorkerName)
	if !ok || inst.Client == nil || inst.Draining {
		return abi.NewErrorResponse(errors.ErrUnavailable), errors.New(errors.CodeContractWorkerUnavailable, "worker unavailable", 503)
	}

	// 3. Serialize request and send over IPC
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request for worker: %w", err)
	}

	frameID := req.ID
	if frameID == "" {
		frameID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	reqFrame := ipc.NewFrame(ipc.FrameTypeRequest, frameID, reqBytes)
	reqFrame.Header.Metadata["handler_id"] = string(entry.Route.ID)
	reqFrame.Header.Metadata["transport"] = string(req.Transport)

	respFrame, err := inst.Client.RoundTrip(ctx, reqFrame)
	if err != nil {
		return abi.NewErrorResponse(errors.ErrUnavailable), fmt.Errorf("worker roundtrip failed: %w", err)
	}

	var resp abi.Response
	if err := json.Unmarshal(respFrame.Body, &resp); err != nil {
		return abi.NewErrorResponse(errors.New(errors.CodeContractResponseInvalid, "malformed response from worker", 502)), err
	}

	return &resp, nil
}

// DispatchStream forwards an incoming streaming request to the worker process.
func (s *Supervisor) DispatchStream(ctx context.Context, req *abi.Request, sink abi.StreamSink, in <-chan []byte) error {
	// 1. Look up route in routing table
	entry, params, ok := s.table.Load(req.Transport, req.Method, req.Path)
	if !ok {
		return errors.ErrNotFound
	}

	req.RoutePath = entry.Route.Path
	if len(params) > 0 {
		if req.Params == nil {
			req.Params = make(map[string]string)
		}
		for k, v := range params {
			req.Params[k] = v
		}
	}

	// 2. Locate worker instance
	inst, ok := s.registry.Get(entry.WorkerName)
	if !ok || inst.Client == nil || inst.Draining {
		return errors.New(errors.CodeContractWorkerUnavailable, "worker unavailable", 503)
	}

	// 3. Serialize request and send over IPC
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request for worker: %w", err)
	}

	streamID := req.ID
	if streamID == "" {
		streamID = fmt.Sprintf("stream-%d", time.Now().UnixNano())
		req.ID = streamID
	}

	streamCh, unregister := inst.Client.RegisterStream(streamID)
	defer unregister()

	startFrame := ipc.NewFrame(ipc.FrameTypeStreamStart, streamID, reqBytes)
	startFrame.Header.Metadata["handler_id"] = string(entry.Route.ID)
	startFrame.Header.Metadata["transport"] = string(req.Transport)

	if err := inst.Client.Send(startFrame); err != nil {
		return fmt.Errorf("send stream start frame: %w", err)
	}

	// Forward client inbound frames if any
	if in != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case data, ok := <-in:
					if !ok {
						return
					}
					dataFrame := ipc.NewFrame(ipc.FrameTypeStreamData, streamID, data)
					_ = inst.Client.Send(dataFrame)
				}
			}
		}()
	}

	defer func() {
		cancelFrame := ipc.NewFrame(ipc.FrameTypeStreamCancel, streamID, nil)
		_ = inst.Client.Send(cancelFrame)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-streamCh:
			if !ok {
				return nil
			}
			switch frame.Header.Type {
			case ipc.FrameTypeStreamData:
				if err := sink.Send(frame.Body); err != nil {
					return err
				}
			case ipc.FrameTypeStreamEnd:
				return nil
			}
		}
	}
}

// DrainAll drains all active workers during mother shutdown.
func (s *Supervisor) DrainAll(ctx context.Context) error {
	workers := s.registry.All()
	var wg sync.WaitGroup

	for _, w := range workers {
		wg.Add(1)
		go func(inst *WorkerInstance) {
			defer wg.Done()
			if inst.Client != nil {
				_ = inst.Client.Send(ipc.NewFrame(ipc.FrameTypeDrain, "drain", nil))
				_ = inst.Client.Close()
			}
			if inst.SocketPath != "" {
				_ = os.Remove(inst.SocketPath)
			}
		}(w)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		s.draining.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
