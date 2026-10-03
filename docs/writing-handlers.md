# Writing Handlers in nxp

The core developer rule of `nxp`:

> **A handler author writes `func Handler(ctx *DOMAIN.Ctx) (Result, error)` and nothing else.**
> They never see `net/http`, socket file descriptors, low-level chunk buffers, or raw schemas.

---

## 1. Directory Tree Routing

Routes are derived directly from the filesystem layout under `define/`:

1. **Path Translation:**
   - Leading `define/` and trailing `.go` are removed.
   - Trailing `index` is dropped: `define/auth/index.go` -> `/auth` (never `/auth/index`).
   - Dynamic parameters `[param]` become `{param}`: `define/items/[id]/index.go` -> `/items/{id}`.
   - Prepend leading `/`.
2. **Filename Prefixes:**
   - `get_`, `post_`, `put_`, `del_`, `patch_` -> REST HTTP verbs.
   - `ws_` -> WebSocket.
   - `grpc_` -> gRPC HTTP/2.
   - `udp_` -> UDP datagram.
   - `mqtt_` -> MQTT subscribe.
   - `nats_` -> NATS subscribe.
   - `kafka_` -> Kafka consumer group.

---

## 2. Handler Directives

First-block doc comments can configure or override routing behaviors:

```go
// Package profile provides member details.
// @route /custom/path
// @transport rest
// @method GET
// @auth required
// @scope profile:read
// @ratelimit 100 200
package profile
```

### Supported Directives:
- `@route <path>`: Override filesystem path.
- `@transport <name>`: `rest`, `websocket`, `grpc`, `webrtc`, `udp`, `mqtt`, `nats`, `kafka`.
- `@method <verb>`: HTTP method or verb.
- `@auth required|optional`: Enforce authentication.
- `@scope <scope1> <scope2>`: Required permission scopes.
- `@ratelimit <rps> <burst>`: Rate limiter settings.
- `@topic <pattern>`: MQTT / PubSub topic (supports wildcards `+`, `#`).
- `@qos <0|1|2>`: Quality of service level.
- `@stream`: Declare durable stream consumer.
- `@group <name>`: Consumer group identifier.
- `@partitions <count>`: Stream partition count.

---

## 3. Domains & Handler Contexts

`nxp` provides specialized, type-safe contexts under `nxp/*`:

### 3.1 REST API Handler
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

### 3.2 File Upload Handler (`nxp/files`)
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

### 3.3 File Download Handler (`nxp/files`)
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

### 3.4 WebSocket Duplex Handler (`nxp/ws`)
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

### 3.5 WebRTC SFU Handler (`nxp/rtc`)
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

### 3.6 MQTT Telemetry Handler (`nxp/mqtt`)
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
