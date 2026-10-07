# Writing Handlers in nxp

The core developer rule of `nxp`:

> **A handler author writes `func Handler(ctx *DOMAIN.Ctx) (Result, error)` and nothing else.**
> They never see `net/http`, socket file descriptors, low-level chunk buffers, or raw schemas.

---

## 1. Directory Tree Routing

Routes are derived directly from the filesystem layout under `define/`.
A route is a **method folder** containing a `handler.go`:

1. **Path Translation:**
   - Leading `define/` and trailing `.go` are removed.
   - The method folder (`get/`, `post/`, ...) and the `handler.go` filename never contribute to the path.
   - Dynamic parameters `@param` (or `[param]`) become `{param}`: `define/items/@id/get/handler.go` -> `/items/{id}`.
   - Prepend leading `/`.
2. **Method Folders:**
   - `get`, `post`, `put`, `patch`, `delete` -> REST HTTP verbs.
   - `ws` -> WebSocket.
   - `rtc` -> WebRTC.
   - `grpc` -> gRPC HTTP/2.
   - `udp` -> UDP datagram.
   - `mqtt` -> MQTT subscribe.
   - `nats` -> NATS subscribe.
   - `kafka` -> Kafka consumer group.

Examples:

```
define/get/handler.go                    -> GET    /
define/agents/get/handler.go             -> GET    /agents
define/agents/post/handler.go            -> POST   /agents
define/agents/@id/get/handler.go         -> GET    /agents/{id}
define/agents/@id/patch/handler.go       -> PATCH  /agents/{id}
define/sources/@id/sync/post/handler.go  -> POST   /sources/{id}/sync
define/chat/room/ws/handler.go           -> WS     /chat/room
define/sensors/telemetry/mqtt/handler.go -> MQTT   /sensors/telemetry
```

> **Why method folders?** Go allows only one `Handler`/`Request`/`Response` per package.
> Putting each method in its own folder (`get/handler.go`, `post/handler.go`) keeps every
> route in its own package, so a path can expose multiple methods without symbol collisions.

---

## 2. Guards

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
// define/agents/@id/get/handler.go
// @guard auth user/admin
package get
```

Guard names are the folder path relative to `define/` (e.g. `auth`, `user/admin`).

---

## 3. Handler Directives

First-block doc comments can configure routing behaviors:

```go
// Package profile provides member details.
// @desc Get the caller profile
// @transport rest
// @auth required
// @scope profile:read
// @guard auth
// @ratelimit 100 200
package get
```

### Supported Directives:
- `@desc <text>` / `@description <text>`: Human-readable endpoint description (MCP-style runtime metadata).
- `@transport <name>`: `rest`, `websocket`, `webrtc`, `grpc`, `udp`, `mqtt`, `nats`, `kafka`.
- `@method <verb>`: HTTP method or verb (normally inferred from the method folder).
- `@auth required|optional`: Enforce authentication.
- `@scope <scope1> <scope2>`: Required permission scopes.
- `@guard <name...>`: Guard implementations to run before the handler.
- `@ratelimit <rps> <burst>`: Rate limiter settings.
- `@topic <pattern>`: MQTT / PubSub topic (supports wildcards `+`, `#`).
- `@qos <0|1|2>`: Quality of service level.
- `@stream`: Declare durable stream consumer.
- `@group <name>`: Consumer group identifier.
- `@partitions <count>`: Stream partition count.

---

## 4. Domains & Handler Contexts

`nxp` provides specialized, type-safe contexts under `nxp/*`:

### 4.1 REST API Handler
```go
package items

import (
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/rest"
)

type ItemResult struct {
    ID    string `json:"id"   validate:"required"`
    Name  string `json:"name" validate:"required"`
}

func Handler(ctx *rest.Ctx) (ItemResult, error) {
    id := ctx.Param("id")
    return ItemResult{ID: id, Name: "Gadget"}, nil
}
```

### 4.2 File Upload Handler (`nxp/files`)
Supports streaming SHA256 calculation, MIME sniffing, virus/quarantine checks, and deduplication:
```go
package upload

import (
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/files"
)

func Handler(ctx *files.UploadCtx) (files.UploadResult, error) {
    if ctx.Quarantine() {
        _ = ctx.Discard()
        return files.UploadResult{}, errors.New(errors.CodeForbidden, "file quarantined", 403)
    }

    if existingID, hit, _ := ctx.Dedupe(); hit {
        _ = ctx.Discard()
        return files.UploadResult{ID: existingID, Size: ctx.Size(), Name: ctx.SafeName()}, nil
    }

    destID := "upload-" + ctx.SafeName()
    _ = ctx.MoveTo(destID)

    return files.UploadResult{
        ID:   destID,
        Size: ctx.Size(),
        Name: ctx.SafeName(),
    }, nil
}
```

### 4.3 File Download Handler (`nxp/files`)
Supports Range, ETag, and RFC 5987 Content-Disposition:
```go
package download

import (
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/files"
)

func Handler(ctx *files.DownloadCtx) (files.DownloadResult, error) {
    return files.DownloadResult{
        Source: ctx.Param("id"),
        Name:   "document.pdf",
    }, nil
}
```

### 4.4 WebSocket Duplex Handler (`nxp/ws`)
```go
package chat

import (
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/ws"
)

type ChatResult struct {
    Status string `json:"status"`
}

func Handler(ctx *ws.Ctx) (ChatResult, error) {
    _ = ctx.WriteJSON(map[string]string{"event": "welcome"})
    return ChatResult{Status: "joined"}, nil
}
```

### 4.5 WebRTC SFU Handler (`nxp/rtc`)
```go
// @transport webrtc
package meet

import (
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/rtc"
)

type MeetResult struct {
    SessionID string `json:"session_id" validate:"required"`
}

func Handler(ctx *rtc.SessionCtx) (MeetResult, error) {
    room := ctx.Room(ctx.Param("room"))
    if track, err := ctx.AcceptTrack(); err == nil && track != nil {
        transcoded := ctx.Transcode(track, rtc.ProfileMedium)
        room.BroadcastTrack(transcoded, "peer-1")
    }
    return MeetResult{SessionID: "sfu-session-1"}, nil
}
```

### 4.6 MQTT Telemetry Handler (`nxp/mqtt`)
```go
// @topic sensors/+/telemetry
// @qos 1
package sensors

import (
    "github.com/Neuxbane/NeuXbaneProtocol/nxp/mqtt"
)

type SensorReading struct {
    SensorID string  `json:"sensor_id" validate:"required"`
    Value    float64 `json:"value"     validate:"min=0"`
}

type SensorAck struct {
    Received bool `json:"received" validate:"required"`
}

func Handler(ctx *mqtt.Ctx[SensorReading, SensorAck]) (SensorAck, error) {
    reading, err := ctx.Payload()
    if err != nil {
        return SensorAck{}, err
    }
    _ = ctx.Ack()
    return SensorAck{Received: true}, nil
}
```
