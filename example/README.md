# nxp Standalone Server

This project runs on the standalone **nxp** backend framework.

## 100% Self-Contained Binary
The `nxp-server` executable embeds everything it needs:
- **Zero external Git dependencies**: Developers do not need to clone or download `github.com/Neuxbane/NeuXbaneProtocol`.
- **Offline compilation**: Handlers and workers compile completely locally using the local `abi/` layer.
- **Single-binary portability**: Share only the `nxp-server` binary. On first run, it scaffolds all required files and documentation.

## Developer Quickstart

All business logic lives inside `define/`.
You do NOT need to write HTTP routers, chunk logic, or schema manifests.

### 1. Handler Signature

Every route handler implements:
```go
func Handler(ctx *rest.Ctx) (Result, error)
```

### 2. Clean Local ABI Imports

Handlers import the server-generated `abi` packages directly:
```go
package define

import (
    "abi/rest"
)

type WelcomeResult struct {
    Message string `json:"message" validate:"required"`
    Status  string `json:"status"  validate:"required"`
}

func Handler(ctx *rest.Ctx) (WelcomeResult, error) {
    return WelcomeResult{
        Message: "Welcome to nxp framework!",
        Status:  "operational",
    }, nil
}
```

Alternatively, you can use the unified import:
```go
import "abi"

func Handler(ctx *abi.Ctx) (WelcomeResult, error)
```

### 3. Running the Server

Start the server:
```bash
./nxp-server -addr :8080 -env dev
```

- Test endpoint: `curl http://localhost:8080/`
- Test introspection: `curl http://localhost:8080/?nxp`
- Hot reload: edit any file in `define/` and save. Changes take effect in <500ms without restarting the server!

See `define/README.md` for the complete guide to writing handlers.
