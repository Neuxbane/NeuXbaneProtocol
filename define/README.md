# Writing Handlers in nxp

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
A route is a **method file** named after the HTTP method (or transport):

- `define/get.go` maps to `GET /`
- `define/agents/get.go` maps to `GET /agents`
- `define/agents/post.go` maps to `POST /agents`
- `define/agents/@id/get.go` maps to `GET /agents/{id}`
- `define/agents/@id/patch.go` maps to `PATCH /agents/{id}`
- `define/sources/@id/sync/post.go` maps to `POST /sources/{id}/sync`

Method file names: `get.go`, `post.go`, `put.go`, `patch.go`, `delete.go`, `ws.go`, `grpc.go`, `udp.go`, `mqtt.go`, `nats.go`, `kafka.go`.
A `@name` folder (or `[name]`) becomes a dynamic path parameter `{name}`.

### Guards

`index.go` is a **guard implementation**, not a route. The folder name is the guard name:

```go
// define/auth/index.go
package auth

func Guard(ctx *rest.Ctx) error { /* ... */ }
```

Endpoints MUST explicitly declare a `// @guard` directive (omitting it fails compilation):
- Specific guards: `// @guard auth` or multiple `// @guard account/auth account/admin`
- Unauthenticated (none): `// @guard` or `// @guard none`

```go
// define/agents/@id/get.go
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
- `enum:"draft,published"`
- `doc:"Human-readable description"`

Supported `validate` rules: `required`, `min=N` / `max=N` (numeric),
`minlen=N` / `maxlen=N` (string length), `pattern=REGEX`,
`enum=a|b|c` / `options=a|b|c`, and the format shortcuts
`email`, `uuid`, `uri`, `url`, `date-time`.

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

   func Handler(ctx *ws.Ctx) (Result, error)
   ```
   `ws.Ctx` exposes `ctx.Request()` plus the embedded `context.Context`.

4. **WebRTC SFU (abi/rtc):**
   ```go
   import "abi/rtc"

   func Handler(ctx *rtc.SessionCtx) (Result, error)
   ```
   `rtc.SessionCtx` exposes `ctx.Request()` plus the embedded `context.Context`.

5. **MQTT Pub/Sub (abi/mqtt):**
   ```go
   import "abi/mqtt"

   func Handler(ctx *mqtt.Ctx[InputEvent, OutputEvent]) (OutputEvent, error)
   ```
   `mqtt.Ctx[In, Out]` exposes the `Payload In` field, `ctx.Request()`, and `ctx.Topic()`.

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

---

## 6. The abi Package Reference

The `abi` package provides the frozen contract between your handlers and the **nxp** runtime engine.

### Unified Imports

You can import domain-specific packages:
```go
import "abi/rest"
import "abi/files"
import "abi/ws"
import "abi/rtc"
import "abi/mqtt"
import "abi/errors"
```

Or import the unified `abi` package directly:
```go
import "abi"

func Handler(ctx *abi.Ctx) (MyResult, error)
```

All types in `abi` are standard Go structs that require no external dependencies.
