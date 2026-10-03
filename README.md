# NeuXbaneProtocol (nxp) — Production Go Backend Framework

**`nxp`** is a high-performance, production-grade Go backend framework that empowers developers to write **only business logic** under a `define/` directory tree, while the framework handles routing, contracts, transports, file handling, WebRTC SFU, hot reload, and introspection.

```go
func Handler(ctx *DOMAIN.Ctx) (Result, error)
```

Developers never touch `net/http`, socket file descriptors, low-level chunk buffers, or raw schemas.

---

## Non-Negotiable Principles

1. **The tree is the config.** `define/**/*.go` defines routes. No route manifests, no `contract.json`. `index.go` maps to the parent path (`/auth/index.go` -> `/auth`), never to `/auth/index`.
2. **Types are the contract.** Go struct tags reflect into JSON Schema Draft 2020-12 at build time via codegen. No hand-written schemas.
3. **The mother enforces, the worker trusts.** Ingress and egress validation happen in the mother process before/after the worker. Workers never validate inbound payloads.
4. **`nxp/*` is the only developer-visible API tree.** `define/*` may import `nxp/*` and stdlib only (enforced in CI).
5. **`internal/*` never imports `define/*`.** The mother learns routes from worker `Hello` frames at runtime.
6. **Adding a transport = adding an adapter.** Never modify `runtime`, `router`, `ipc`, `worker`, or `contract` to add a transport.
7. **Every dependency cycle is a bug.** Enforce dependency acyclicity in CI.
8. **Hot reload is a rebuild + Hello + atomic route swap + graceful drain.** No in-process plugin loading, ever.

---

## Architecture Overview

- **Mother Process:** Owns listener sockets, config, contract validation, routing tables, and worker supervisors.
- **Worker Processes:** Subprocesses running business handlers linked with the generated route registry. Communicate over framed Unix domain sockets.
- **Codegen (`nxp-codegen`):** Walks `define/`, inspects handler AST signatures, calculates build content hashes, and generates `internal/generated/routes_gen.go`.
- **Introspection (`?nxp`):** Intercepted in Mother before route dispatch, serving `application/json`, `text/plain`, `text/html`, `application/openapi+json`, `application/asyncapi+json`, and `/__nxp/schema`.
- **Transports:** Pluggable adapters for REST, WebSocket, gRPC, UDP, WebRTC SFU, MQTT, NATS, AMQP, Redis, and Kafka.

---

## Getting Started

### 1. Build & Run Codegen
```bash
go run ./cmd/nxp-codegen
```

### 2. Build the Binaries
```bash
go build -o bin/nxp-server ./cmd/nxp-server
go build -o bin/nxp-worker ./cmd/nxp-worker
```

### 3. Run the Server
```bash
./bin/nxp-server -addr :8080 -env dev
```

### 4. Run the Test Suite
```bash
go test ./...
```

---

## Documentation

- [Architecture & Design](docs/architecture.md)
- [Contracts & Schema Enforcement](docs/contracts.md)
- [Writing Handlers](docs/writing-handlers.md)
- [Transports & Adapters](docs/transports.md)
