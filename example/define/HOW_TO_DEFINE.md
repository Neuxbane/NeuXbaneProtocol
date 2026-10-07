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

The directory structure under `define/` directly defines your routes.
A route is a **method folder** containing a `handler.go`:

- `define/get/handler.go` maps to `GET /`
- `define/agents/get/handler.go` maps to `GET /agents`
- `define/agents/post/handler.go` maps to `POST /agents`
- `define/agents/@id/get/handler.go` maps to `GET /agents/{id}`
- `define/agents/@id/patch/handler.go` maps to `PATCH /agents/{id}`
- `define/sources/@id/sync/post/handler.go` maps to `POST /sources/{id}/sync`

Method folders: `get`, `post`, `put`, `patch`, `delete`, `ws`, `rtc`, `grpc`, `udp`, `mqtt`, `nats`, `kafka`.
A `@name` folder becomes a dynamic path parameter `{name}`.

### Guards

`index.go` is a **guard implementation**, not a route. The folder name is the guard name:

```go
// define/auth/index.go
package auth

func Guard(ctx *rest.Ctx) error { /* ... */ }
```

Endpoints opt in with a directive in their handler file:

```go
// define/agents/@id/get/handler.go
// @guard auth user/admin
package get
```

### Descriptions

Every endpoint should declare a description (MCP-style runtime metadata):

```go
// @desc List all agents
package get
```

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
