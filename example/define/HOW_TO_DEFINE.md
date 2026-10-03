# How to Write Handlers in nxp

Welcome to **nxp**! In this framework, you write **only business logic**.
Routing, schema reflection, validation, transports, WebRTC SFU, hot reload, and introspection are handled automatically.

---

## 1. The Core Rule

Every handler must implement the exact signature:

```go
func Handler(ctx *DOMAIN.Ctx) (Result, error)
```

You never see `net/http`, socket file descriptors, low-level chunk buffers, or raw schemas.

---

## 2. Directory Tree Routing

The directory structure under `define/` directly defines your routes:

- `define/index.go` maps to `GET /`
- `define/auth/index.go` maps to `GET /auth` (never `/auth/index`)
- `define/items/item/index.go` with `// @route /items/{id}` maps to `GET /items/{id}`
- `define/items/create/index.go` with `// @method POST` maps to `POST /items/create`

Filename prefixes can also specify method and transport:
- `get_`, `post_`, `put_`, `del_`, `patch_` -> REST HTTP verbs
- `ws_` -> WebSocket
- `grpc_` -> gRPC HTTP/2
- `udp_` -> UDP datagram
- `mqtt_` -> MQTT subscriber
- `nats_` -> NATS subscriber
- `kafka_` -> Kafka consumer

---

## 3. Types are the Contract

Struct tags are reflected into JSON Schema Draft 2020-12 at build time:
- `json:"field_name"`
- `validate:"required,min=1,max=100"`
- `format:"email"` or `format:"date-time"`
- `doc:"Human-readable description"`

Ingress validation runs in the Mother process before your handler is ever called.
If a request fails validation, a 400 error is returned immediately and your worker is not invoked.

---

## 4. Available Domains (via server-generated abi/)

Import from `abi/*` or unified `abi`:

1. **REST APIs (abi/rest):**
   ```go
   import "abi/rest"

   func Handler(ctx *rest.Ctx) (Result, error)
   ```
   Available methods on `ctx`:
   - `ctx.Param("id")` - path parameter
   - `ctx.Query("sort")` - query parameter
   - `ctx.Header("Authorization")` - header
   - `ctx.Bind(&data)` - binds JSON body
   - `ctx.Identity()` - authenticated user (subject, scopes, roles)

2. **File Uploads / Downloads (abi/files):**
   ```go
   import "abi/files"

   func Handler(ctx *files.UploadCtx) (files.UploadResult, error)
   ```
   Methods:
   - `ctx.SHA256()`, `ctx.Size()`, `ctx.Mime()`, `ctx.Reader()`, `ctx.MoveTo(dest)`

3. **WebSockets (abi/ws):**
   ```go
   import "abi/ws"

   func Handler(ctx *ws.Ctx) error
   ```

4. **WebRTC SFU (abi/rtc):**
   ```go
   import "abi/rtc"

   func Handler(ctx *rtc.SessionCtx) error
   ```

5. **MQTT Pub/Sub (abi/mqtt):**
   ```go
   import "abi/mqtt"

   func Handler(ctx *mqtt.Ctx[InputEvent, OutputEvent]) (OutputEvent, error)
   ```

---

## 5. Serving Files, HTML, CSS, Video, and S3 Assets

Handlers are not limited to JSON. You can return HTML, CSS, images, videos with byte-range seeking, database blobs, or S3 presigned downloads:

### A. Returning HTML or CSS
```go
func Handler(ctx *rest.Ctx) (string, error) {
    return ctx.HTML("<h1>Welcome to nxp!</h1>")
}

func Handler(ctx *rest.Ctx) (string, error) {
    return ctx.CSS("body { background: #111; color: #fff; }")
}
```

### B. Streaming Video & Audio (with Range & Seeking Support)
The framework automatically handles HTTP 206 Partial Content byte ranges, ETags, and streaming without loading the entire video into memory:
```go
func Handler(ctx *rest.Ctx) (any, error) {
    return ctx.File("./assets/video.mp4")
}
```

### C. Serving In-Memory Images or Database Blobs
```go
func Handler(ctx *rest.Ctx) (any, error) {
    pngBytes := fetchImageFromDatabase() // []byte
    return ctx.Blob("image/png", pngBytes)
}
```

### D. Download as Attachment
```go
func Handler(ctx *rest.Ctx) (any, error) {
    return ctx.Download("./reports/annual.pdf", "Annual_Report_2026.pdf")
}
```

### E. S3 Presigned URLs & Redirects
```go
import "abi/files"

func Handler(ctx *rest.Ctx) (files.DownloadResult, error) {
    return files.DownloadResult{
        Source:    "https://my-bucket.s3.amazonaws.com/large-asset.zip?...",
        Presigned: true,
    }, nil
}
```
