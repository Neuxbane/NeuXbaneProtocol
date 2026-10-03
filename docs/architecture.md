# nxp Architecture & Design

The **`nxp`** framework is a production-grade Go backend framework designed around developer ergonomics, process isolation, schema-driven contracts, and zero-downtime hot reload.

---

## 1. High-Level Topology

```
                  +----------------------------------------------+
                  |                 MOTHER PROCESS               |
                  |                                              |
 Inbound Traffic  |  +----------------+     +-----------------+  |
 [HTTP, WS, gRPC, |  |   Transports   |     |    Contracts    |  |
  UDP, MQTT, ...] |  |  (Adapters)    |---->| (Ingress Val.)  |  |
        |         |  +----------------+     +--------+--------+  |
        v         |                                  |           |
 [Listeners] ---->|  +----------------+              |           |
                  |  | Routing Table  |              v           |
                  |  | (Atomic Swaps) |     +-----------------+  |
                  |  +----------------+     |  IPC Multiplex  |  |
                  |                         |  (UNIX Sockets) |  |
                  +-------------------------+--------+--------+--+
                                                     |
                                   +-----------------+-----------------+
                                   |                                   |
                                   v                                   v
                      +-------------------------+         +-------------------------+
                      |     WORKER PROCESS 1    |         |     WORKER PROCESS 2    |
                      |   (Serving / Draining)  |         |      (Active / Fresh)   |
                      |                         |         |                         |
                      |  +-------------------+  |         |  +-------------------+  |
                      |  | Handler Registry  |  |         |  | Handler Registry  |  |
                      |  | (Generated Routes)|  |         |  | (Generated Routes)|  |
                      |  +-------------------+  |         |  +-------------------+  |
                      |  | Business Handlers |  |         |  | Business Handlers |  |
                      |  | (define/**/*.go)  |  |         |  | (define/**/*.go)  |  |
                      +-------------------------+         +-------------------------+
```

---

## 2. Core Principles

1. **The tree is the config:** `define/**/*.go` defines the routing topology. There are no route manifests or configuration files. `index.go` maps directly to its parent route (`/auth/index.go` -> `/auth`), never to `/auth/index`.
2. **Types are the contract:** Go struct tags reflect into JSON Schema Draft 2020-12 at build time via codegen. No handwritten schema files.
3. **The mother enforces, the worker trusts:** Ingress and egress contract validation execute strictly inside the mother process before and after dispatching to workers. Workers never validate inbound payloads.
4. **`nxp/*` is the only developer-visible API tree:** Handlers under `define/*` may import only `nxp/*` and the Go standard library.
5. **`internal/*` never imports `define/*`:** The mother runtime discovers routes dynamically from worker `Hello` frames at startup.
6. **Adding a transport = adding an adapter:** New network protocols are introduced by implementing `transport.Adapter` without touching `runtime`, `router`, `ipc`, `worker`, or `contract`.
7. **Every dependency cycle is a bug:** Package dependencies are acyclic, enforced via static analysis in CI (`test/arch_test.go`).
8. **Hot reload is rebuild + Hello + atomic swap + graceful drain:** Code changes trigger a background rebuild; the new worker connects, exchanges handshake frames, replaces routing pointers atomically, and the old worker drains in-flight requests.

---

## 3. Worker Design Decision

### Single Generic Worker Linked via Generated Registry

Rather than compiling a separate binary for each route directory or attempting in-process dynamic plugin loading (which is notoriously fragile in Go across platforms), `nxp` compiles a single worker binary `nxp-worker` that links the generated route registry (`internal/generated/routes_gen.go`).

**Rationale:**
- **Zero In-Process Plugin Fragility:** Go's `plugin` package suffers from symbol table conflicts, CGO requirements, and allocator inconsistencies. Subprocess workers ensure 100% crash isolation.
- **Fast Build Times (<500ms):** Only changed handler packages and the generated route table require compilation; the worker runtime infrastructure is reused without modification.
- **Atomic Route Swapping:** The mother process maintains a lock-free/RWMutex route table. When a new worker completes its `Hello` handshake, route pointers swap instantly.

---

## 4. Framed IPC Protocol

Communication between Mother and Workers uses Unix Domain Sockets (`WORKER_SOCK`) with a binary framing protocol:

```
+------------------------------------+--------------------------+---------------------+
| 4-byte Big-Endian Length (uint32) | JSON Header (variable)   | Raw Body (variable) |
+------------------------------------+--------------------------+---------------------+
```

### Frame Types:
- `hello`: Worker registers its PID, `BuildID`, and route list with Mother.
- `ready`: Mother acknowledges successful registration and begins routing.
- `req`: Ingress request frame forwarded to worker.
- `resp`: Worker execution response returned to Mother.
- `drain`: Mother orders worker to stop accepting new requests, finish in-flight work, and terminate.
- `ping` / `pong`: Liveness check.
