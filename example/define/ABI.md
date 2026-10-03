# Application Binary Interface (abi) Reference

The `abi` package provides the frozen contract between your handlers and the **nxp** runtime engine.

## Unified Imports

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
