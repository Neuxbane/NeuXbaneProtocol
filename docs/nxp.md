# nxp — The Complete Guide

**nxp** is a production-grade Go backend framework built around four ideas:

1. **The tree is the config** — the `define/` directory layout *is* your routing table. No manifests, no annotations to register routes.
2. **Types are the contract** — Go struct tags reflect into JSON Schema Draft 2020-12 at build time. No handwritten schemas.
3. **The mother enforces, the worker trusts** — ingress/egress validation runs in the mother process; workers only run business logic.
4. **Hot reload is rebuild + handshake + atomic swap + graceful drain** — edit a file, save, and the new worker takes over in <500ms.

This single document replaces the former `writing-handlers.md`, `architecture.md`, `transports.md`, and `contracts.md`.

---

## Table of Contents

1. [The Core Rule](#1-the-core-rule)
2. [Directory Tree Routing](#2-directory-tree-routing)
3. [Guards](#3-guards)
4. [Handler Directives](#4-handler-directives)
5. [Types are the Contract](#5-types-are-the-contract)
6. [Handler Contexts (the `abi/*` API)](#6-handler-contexts-the-abi-api)
7. [Serving Files, HTML, CSS, and Assets](#7-serving-files-html-css-and-assets)
8. [Architecture](#8-architecture)
9. [Transports & Adapters](#9-transports--adapters)
10. [Contracts & Enforcement](#10-contracts--enforcement)

---

## 1. The Core Rule

> **A handler author writes `func Handler(ctx *DOMAIN.Ctx) (Result, error)` and nothing else.**
> They never see `net/http`, socket file descriptors, low-level chunk buffers, or raw schemas.

Every route handler implements the same shape, differing only in the context type:

```go
func Handler(ctx *rest.Ctx) (Result, error)   // REST
func Handler(ctx *ws.Ctx) (Result, error)     // WebSocket
func Handler(ctx *rtc.SessionCtx) (Result, error) // WebRTC
```

The `Result` type is any Go struct; its tags become the response schema.

---

## 2. Directory Tree Routing

Routes are derived directly from the filesystem layout under `define/`. There are **three** accepted authoring forms — `handler.go` is **not required**.

### Form A — Bare method file (recommended)

The filename itself is the method:

```
define/get.go                    -> GET     /
define/agents/get.go             -> GET     /agents
define/agents/post.go            -> POST    /agents
define/agents/@id/get.go         -> GET     /agents/{id}
define/agents/@id/patch.go       -> PATCH   /agents/{id}
define/some/socket/ws.go         -> CONNECT /some/socket
define/sensors/telemetry/mqtt.go -> SUB     /sensors/telemetry
```

### Form B — Method folder + `handler.go`

The folder is the method and `handler.go` is the handler:

```
define/get/handler.go                    -> GET    /
define/agents/get/handler.go             -> GET    /agents
define/agents/@id/get/handler.go         -> GET    /agents/{id}
define/sources/@id/sync/post/handler.go  -> POST   /sources/{id}/sync
define/chat/room/ws/handler.go           -> CONNECT /chat/room
```

### Form C — Legacy prefix file

```
define/get_user.go    -> GET     /user
define/post_login.go  -> POST    /login
define/ws_stream.go   -> CONNECT /stream
define/mqtt_events.go -> SUB     /events
```

### Path translation rules

- Leading `define/` and trailing `.go` are removed.
- A **method segment** (`get`, `post`, `put`, `patch`, `delete`/`del`, `ws`, `rtc`, `grpc`, `udp`, `mqtt`, `nats`, `kafka`) never contributes to the path — whether it is a folder or a filename.
- Dynamic parameters use `@param` **or** `[param]` and become `{param}`: `define/items/@id/get.go` -> `/items/{id}`.
- A leading `/` is prepended.

### Method → transport mapping

| Segment | Transport | Method |
| :--- | :--- | :--- |
| `get` | REST | `GET` |
| `post` | REST | `POST` |
| `put` | REST | `PUT` |
| `patch` | REST | `PATCH` |
| `delete`, `del` | REST | `DELETE` |
| `ws` | WebSocket | `CONNECT` |
| `rtc` | WebRTC | `CONNECT` |
| `grpc` | gRPC | `POST` |
| `udp` | UDP | `SEND` |
| `mqtt` | MQTT | `SUB` |
| `nats` | NATS | `SUB` |
| `kafka` | Kafka | `CONSUME` |

> **Why method folders?** Go allows only one `Handler`/`Request`/`Response` per package.
> Putting each method in its own folder (`get/handler.go`, `post/handler.go`) keeps every
> route in its own package, so a path can expose multiple methods without symbol collisions.
> When you author a bare method file (`get.go`), the framework stages it into a method
> folder (`get/handler.go`) automatically — you never have to do this yourself.

---

## 3. Guards

`index.go` is a **guard implementation**, not a route. The folder name is the guard name:

```go
// define/auth/index.go
package auth

import (
    "abi/errors"
    "abi/rest"
)

func Guard(ctx *rest.Ctx) error {
    id := ctx.Identity()
    if id == nil || !id.IsAuthenticated() {
        return errors.New("unauthorized", "authentication required", 401)
    }
    return nil
}
```

Endpoints opt in with the `@guard` directive in their handler file:

```go
// define/agents/@id/get.go
// @guard auth user/admin
package get
```

Guard names are the folder path relative to `define/` (e.g. `auth`, `user/admin`).

> **Required directive:** every route file **must** declare a `@guard` directive.
> Use `// @guard auth`, `// @guard auth user/admin`, or a bare `// @guard` for no guards.
> A route file without `@guard` fails codegen with:
> `route <path>: missing required @guard directive`.

---

## 4. Handler Directives

First-block doc comments configure routing behavior:

```go
// Package profile provides member details.
// @desc Get the caller profile
// @transport rest
// @method GET
// @auth required
// @scope profile:read
// @guard auth
// @ratelimit 100 200
package get
```

### Supported directives

| Directive | Purpose |
| :--- | :--- |
| `@desc <text>` / `@description <text>` | Human-readable endpoint description (MCP-style runtime metadata). |
| `@transport <name>` | Override the inferred transport. Valid values: `rest`, `grpc`, `graphql`, `websocket`, `sse`, `webtransport`, `webrtc`, `udp`, `mqtt`, `nats`, `amqp`, `redis`, `kafka`. Invalid values are ignored. |
| `@method <verb>` | Override the inferred method (uppercased). |
| `@auth required\|optional\|public` | Authentication policy. |
| `@scope <s1> <s2>` / `@scopes ...` | Required permission scopes. |
| `@guard <name...>` / `@guards ...` | Guard implementations to run before the handler. Space- or comma-separated; `none` or bare means no guards. |
| `@ratelimit <rps> <burst>` | Rate limiter settings. |
| `@topic <pattern>` | MQTT / PubSub topic (supports wildcards `+`, `#`). |
| `@qos <0\|1\|2>` | Quality of service level. |
| `@retain` | Retain the last message on the topic. |
| `@stream` | Declare a durable stream consumer. |
| `@group <name>` | Consumer group identifier. |
| `@partitions <count>` | Stream partition count. |

> **Precedence:** `@transport` and `@method` **override** the value inferred from the
> folder/filename. If omitted, the transport and method come from the tree.

---

## 5. Types are the Contract

Struct tags are reflected into **JSON Schema Draft 2020-12** at build time:

| Tag | Purpose | Example |
| :--- | :--- | :--- |
| `json` | Serialized field name and omission rules | `json:"user_id,omitempty"` |
| `validate` | Validation constraints (see below) | `validate:"required,min=1"` |
| `format` | JSON Schema semantic format | `format:"email"`, `format:"date-time"` |
| `enum` | Allowed values (comma- or `\|`-separated) | `enum:"draft,published"` |
| `doc` | Schema field documentation | `doc:"Unique identifier for the item"` |

Supported `validate` rules:

| Rule | Effect |
| :--- | :--- |
| `required` | Field is added to the schema's `required` list. |
| `min=N` / `max=N` | Numeric `minimum` / `maximum`. |
| `minlen=N` / `maxlen=N` | String `minLength` / `maxLength`. |
| `pattern=REGEX` | String `pattern`. |
| `enum=a\|b\|c` / `options=a\|b\|c` | Allowed values. |
| `email`, `uuid`, `uri`, `url`, `ipv4`, `ipv6`, `date-time` | Sets the JSON Schema `format`. |

```go
type CreateItemPayload struct {
    Name  string `json:"name"  validate:"required,minlen=1,maxlen=100"`
    Price int    `json:"price" validate:"min=1"`
    State string `json:"state" enum:"draft,published"`
}

type CreateItemResult struct {
    ID    string `json:"id"    validate:"required"`
    Name  string `json:"name"  validate:"required"`
    Price int    `json:"price" validate:"min=1"`
}
```

Ingress validation runs in the **Mother** process before your handler is ever called.
If a request fails validation, a `400` is returned immediately and your worker is not invoked.

---

## 6. Handler Contexts (the `abi/*` API)

Handlers import the **server-generated local `abi/*` packages** (or the unified `abi`
package) plus the Go standard library. The `nxp-server` binary scaffolds these packages
into your project on first run — you never download the framework to write handlers.

```go
import (
    "abi"          // unified core types (Ctx, Request, Response, Identity, Error, ...)
    "abi/rest"     // REST context alias
    "abi/files"    // upload / download contexts
    "abi/ws"       // WebSocket context
    "abi/rtc"      // WebRTC SFU context
    "abi/mqtt"     // MQTT context
    "abi/errors"   // error constructors
)
```

### 6.1 REST (`abi/rest`)

`abi/rest.Ctx` is an alias of `abi.Ctx`.

```go
package items

import "abi/rest"

type ItemResult struct {
    ID   string `json:"id"   validate:"required"`
    Name string `json:"name" validate:"required"`
}

func Handler(ctx *rest.Ctx) (ItemResult, error) {
    id := ctx.Param("id")
    return ItemResult{ID: id, Name: "Gadget"}, nil
}
```

Context methods:

| Method | Purpose |
| :--- | :--- |
| `ctx.Request()` / `ctx.Response()` | Raw `*abi.Request` / `*abi.Response` |
| `ctx.Param("id")` | Path parameter |
| `ctx.Query("sort")` | Query parameter |
| `ctx.QueryAll("tag")` | All values for a repeated query parameter |
| `ctx.Header("Authorization")` | Request header |
| `ctx.RawBody()` | Raw request body bytes |
| `ctx.Bind(&data)` | Bind JSON body into a struct |
| `ctx.Identity()` | Authenticated identity (`*abi.Identity`) |
| `ctx.Status(code)` | Set the response status code |
| `ctx.SetHeader(k, v)` | Set a response header |
| `ctx.HTML(html)` / `ctx.Text(t)` / `ctx.CSS(css)` | Return HTML / plain text / CSS |
| `ctx.Blob(mime, data)` | Return in-memory bytes with a MIME type |
| `ctx.File(path)` | Stream a file (Range/ETag aware) |
| `ctx.Download(path, name)` | Stream a file as an attachment |
| `ctx.Redirect(url, code)` | Redirect the client |

### 6.2 WebSocket (`abi/ws`)

`abi/ws.Ctx` is an alias of `abi.WsCtx`. The context exposes the underlying request; the
runtime owns the socket framing.

```go
package chat

import "abi/ws"

type ChatResult struct {
    Status string `json:"status"`
}

func Handler(ctx *ws.Ctx) (ChatResult, error) {
    req := ctx.Request() // *abi.Request (headers, query, identity, ...)
    _ = req
    return ChatResult{Status: "joined"}, nil
}
```

Context methods: `ctx.Request()` (plus the embedded `context.Context`).

### 6.3 File Upload (`abi/files`)

```go
package upload

import (
    "abi/errors"
    "abi/files"
)

func Handler(ctx *files.UploadCtx) (files.UploadResult, error) {
    if ctx.Quarantine() {
        _ = ctx.Discard()
        return files.UploadResult{}, errors.New("forbidden", "file quarantined", 403)
    }
    if existingID, hit, _ := ctx.Dedupe(); hit {
        _ = ctx.Discard()
        return files.UploadResult{ID: existingID, Size: ctx.Size(), Name: ctx.SafeName()}, nil
    }
    destID := "upload-" + ctx.SafeName()
    _ = ctx.MoveTo(destID)
    return files.UploadResult{ID: destID, Size: ctx.Size(), Name: ctx.SafeName()}, nil
}
```

Methods: `ctx.SHA256()`, `ctx.Size()`, `ctx.Mime()`, `ctx.OriginalName()`, `ctx.SafeName()`,
`ctx.Quarantine()`, `ctx.Field(name)`, `ctx.Reader()`, `ctx.MoveTo(dest)`, `ctx.CopyTo(dest)`,
`ctx.Discard()`, `ctx.Dedupe()`.

### 6.4 File Download (`abi/files`)

```go
package download

import "abi/files"

func Handler(ctx *files.DownloadCtx) (files.DownloadResult, error) {
    return files.DownloadResult{
        Source: ctx.ID(),
        Name:   "document.pdf",
    }, nil
}
```

`DownloadCtx` exposes `ctx.ID()`. `DownloadResult` fields: `Source`, `Name`, `Presigned`.

### 6.5 WebRTC SFU (`abi/rtc`)

`abi/rtc.SessionCtx` is an alias of `abi.SessionCtx`.

```go
// @transport webrtc
package meet

import "abi/rtc"

type MeetResult struct {
    SessionID string `json:"session_id" validate:"required"`
}

func Handler(ctx *rtc.SessionCtx) (MeetResult, error) {
    _ = ctx.Request()
    return MeetResult{SessionID: "sfu-session-1"}, nil
}
```

Context methods: `ctx.Request()` (plus the embedded `context.Context`). The `rtc` package
also re-exports `Peer`, `Room`, `Track`, `VideoProfile`, and the `ProfileHigh` /
`ProfileMedium` / `ProfileLow` constants.

### 6.6 MQTT (`abi/mqtt`)

```go
// @topic sensors/+/telemetry
// @qos 1
package sensors

import "abi/mqtt"

type SensorReading struct {
    SensorID string  `json:"sensor_id" validate:"required"`
    Value    float64 `json:"value"     validate:"min=0"`
}

type SensorAck struct {
    Received bool `json:"received" validate:"required"`
}

func Handler(ctx *mqtt.Ctx[SensorReading, SensorAck]) (SensorAck, error) {
    reading := ctx.Payload // typed input, already bound from the message body
    _ = reading
    return SensorAck{Received: true}, nil
}
```

`mqtt.Ctx[In, Out]` exposes the `Payload In` **field** (bound from the message body),
`ctx.Request()`, and `ctx.Topic()`.

---

## 7. Serving Files, HTML, CSS, and Assets

Handlers are not limited to JSON. You can return HTML, CSS, images, videos with byte-range
seeking, database blobs, or S3 presigned downloads.

### A. HTML or CSS

```go
func Handler(ctx *rest.Ctx) (string, error) {
    return ctx.HTML("<h1>Welcome to nxp!</h1>")
}

func Handler(ctx *rest.Ctx) (string, error) {
    return ctx.CSS("body { background: #111; color: #fff; }")
}
```

### B. Streaming video & audio (Range + seeking)

The framework handles HTTP 206 Partial Content, ETags, and streaming without loading the
whole file into memory:

```go
func Handler(ctx *rest.Ctx) (any, error) {
    return ctx.File("./assets/video.mp4")
}
```

### C. In-memory images or database blobs

```go
func Handler(ctx *rest.Ctx) (any, error) {
    pngBytes := fetchImageFromDatabase() // []byte
    return ctx.Blob("image/png", pngBytes)
}
```

### D. Download as attachment

```go
func Handler(ctx *rest.Ctx) (any, error) {
    return ctx.Download("./reports/annual.pdf", "Annual_Report_2026.pdf")
}
```

### E. S3 presigned URLs & redirects

```go
import "abi/files"

func Handler(ctx *rest.Ctx) (files.DownloadResult, error) {
    return files.DownloadResult{
        Source:    "https://my-bucket.s3.amazonaws.com/large-asset.zip?...",
        Presigned: true,
    }, nil
}
```

---

## 8. Architecture

### 8.1 High-level topology

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
                      |  +-------------------+  |         |  +-------------------+  |
                      |  | Handler Registry  |  |         |  | Handler Registry  |  |
                      |  | (Generated Routes)|  |         |  | (Generated Routes)|  |
                      |  +-------------------+  |         |  +-------------------+  |
                      |  | Business Handlers |  |         |  | Business Handlers |  |
                      |  | (define/**/*.go)  |  |         |  | (define/**/*.go)  |  |
                      +-------------------------+         +-------------------------+
```

### 8.2 Core principles

1. **The tree is the config:** `define/**/*.go` defines the routing topology. There are no route manifests or config files.
2. **Types are the contract:** Go struct tags reflect into JSON Schema Draft 2020-12 at build time via codegen.
3. **The mother enforces, the worker trusts:** Ingress and egress validation run strictly inside the mother process before and after dispatching to workers.
4. **`abi/*` is the only developer-visible API tree:** Handlers under `define/*` may import only the server-generated `abi/*` packages and the Go standard library.
5. **`internal/*` never imports `define/*`:** The mother discovers routes dynamically from worker `Hello` frames at startup.
6. **Adding a transport = adding an adapter:** New protocols are introduced by implementing `transport.Adapter` without touching `runtime`, `router`, `ipc`, `worker`, or `contract`.
7. **Every dependency cycle is a bug:** Package dependencies are acyclic, enforced via static analysis in CI (`test/arch_test.go`).
8. **Hot reload is rebuild + Hello + atomic swap + graceful drain.**

### 8.3 Worker design

Rather than compiling a separate binary per route directory or using in-process plugins
(fragile in Go across platforms), `nxp` compiles a **single generic worker** (`nxp-worker`)
that links the generated route registry (`internal/generated/routes_gen.go`).

- **Crash isolation:** subprocess workers give 100% isolation.
- **Fast builds (<500ms):** only changed handler packages and the generated route table recompile.
- **Atomic route swapping:** the mother holds a lock-free/RWMutex route table; when a new worker completes its `Hello` handshake, route pointers swap instantly.

### 8.4 Framed IPC protocol

Mother ↔ Worker communication uses Unix Domain Sockets (`WORKER_SOCK`) with binary framing:

```
+------------------------------------+--------------------------+---------------------+
| 4-byte Big-Endian Length (uint32) | JSON Header (variable)   | Raw Body (variable) |
+------------------------------------+--------------------------+---------------------+
```

Frame types: `hello`, `ready`, `req`, `resp`, `drain`, `ping`, `pong`.

---

## 9. Transports & Adapters

`nxp` decouples business logic from network transport through the **Adapter Architecture**.

> **Principle 6:** Adding a transport = adding an adapter. Never modify `runtime`, `router`,
> `ipc`, `worker`, or `contract` to add a transport.

### 9.1 The `transport.Adapter` interface

```go
type Adapter interface {
    Name() abi.Transport
    MatchKey() string
    Normalize(raw any) (*abi.Request, error)
    Render(w io.Writer, resp *abi.Response, ctx RenderCtx) error
    Serve(listener *net.Listener, table *router.Table) error
    Shutdown(ctx context.Context) error
}
```

- `Name()` — canonical `abi.Transport` enum value.
- `MatchKey()` — dispatch lookup pattern (e.g. `rest:GET:/path`, `mqtt:sensors/+/telemetry`).
- `Normalize(raw any)` — maps a protocol-native connection/message into a transport-neutral `*abi.Request`.
- `Render(...)` — formats a generic `*abi.Response` into protocol-specific wire bytes (ETag, Range, QoS acks, status codes).
- `Serve(...)` — runs the protocol listening loop.
- `Shutdown(...)` — gracefully stops the listener and closes active connections.

### 9.2 Implemented transports

| Transport | Package | Features |
| :--- | :--- | :--- |
| **REST** | `internal/transport/rest` | HTTP/1.1, HTTP/2, ETag caching, Range / partial content (206), conditional GET (304), chunked bodies |
| **WebSocket** | `internal/transport/websocket` | Duplex framing, heartbeat ping/pong, backpressure buffering, resume tokens |
| **gRPC** | `internal/transport/grpc` | HTTP/2 binary framing, reflection from Go types without handwritten proto files |
| **UDP** | `internal/transport/udp` | Datagram routing, response modes: `none`, `echo`, `unicast` |
| **WebRTC** | `internal/transport/webrtc` | Pion WebRTC v4 SFU, audio/video track forwarding, dynamic profile transcoding (1080p, 720p, 360p) |
| **MQTT** | `internal/transport/mqtt` | MQTT 3.1.1 packet parsing, topic wildcards (`+`, `#`), QoS 0/1/2, retain, will |
| **NATS** | `internal/transport/nats` | Subject routing, reply subjects, JetStream consumer groups |
| **AMQP** | `internal/transport/amqp` | Exchanges, queues, routing keys, publisher confirms |
| **Redis** | `internal/transport/redis` | Pub/Sub channels and Redis Streams with consumer groups |
| **Kafka** | `internal/transport/kafka` | Topic partitions, consumer groups, offset commits, DLQ |

### 9.3 Conformance invariant

Every adapter must pass `internal/transport/conformance/conformance_test.go`:
normalization produces valid headers/path/query/payload; rendering respects status codes
and headers; the dispatcher propagates responses and errors. Adapters self-register:

```go
func init() {
    transport.RegisterAdapter(NewAdapter(nil))
}
```

---

## 10. Contracts & Enforcement

### 10.1 Polymorphic route shapes

Every route defines its contract using one of five `abi.Shape` implementations:

1. **`RequestResponseShape`** — unary request/response (REST, gRPC). `Request *abi.Schema`, `Responses map[int]*abi.Schema`.
2. **`FramesShape`** — bidirectional streams of message frames (WebSocket, WebRTC DataChannels). `In`, `Out *abi.Schema`.
3. **`DatagramShape`** — lossy/fast datagrams (UDP). `Body *abi.Schema`, `MaxSize int`, `Response DatagramResponse` (`none`, `echo`, `unicast`).
4. **`PubSubShape`** — topic-routed broker streams (MQTT, AMQP, Redis Pub/Sub). `Topics map[string]string`, `QoS int`, `Retain bool`, `In`, `Out *abi.Schema`.
5. **`StreamShape`** — partitioned durable event logs (Kafka, NATS JetStream, Redis Streams). `Partitions int`, `ConsumerGroup string`, `In`, `Out *abi.Schema`.

### 10.2 Mother enforcement policy

- **Ingress validation:** before a request is forwarded to a worker, the mother validates query params, headers, and payload against the route's `Shape`. If invalid, an error is returned immediately (`contract.request.invalid`) and the worker is never invoked.
- **Egress validation:** after a worker completes, the mother validates the result payload and status code.

Egress violation policies (`egress_policy`):

| Policy | Behavior |
| :--- | :--- |
| `warn` | Log the discrepancy and emit metrics; pass the response to the client. |
| `quarantine` *(default)* | Log a critical violation, mark the worker suspicious, replace the response with a safe sanitized error (`contract.response.invalid`). |
| `kill` | Immediately terminate the offending worker with SIGKILL and restart a fresh instance. |

### 10.3 Structured violations & error codes

| Error Code | HTTP Status | Description |
| :--- | :--- | :--- |
| `contract.request.invalid` | 400 | Inbound payload or parameter failed schema validation |
| `contract.request.unauthorized` | 401 | Missing or invalid authentication token |
| `contract.request.forbidden` | 403 | Authenticated identity lacks required scope |
| `contract.request.ratelimited` | 429 | Rate limit exceeded |
| `contract.response.invalid` | 502 | Worker produced a payload violating the route schema |
| `contract.response.statusnotallowed` | 502 | Worker returned a status code not declared in the route's `Responses` map |
| `contract.worker.unavailable` | 503 | Worker socket disconnected or crashed |

### 10.4 Schema bundles

The full schema for the active build is queryable:

```bash
curl "http://localhost:8080/__nxp/schema?nxp&build=<build-id>"
```

```http
Content-Type: application/schema+json; charset=utf-8
Cache-Control: public, max-age=31536000, immutable
```
