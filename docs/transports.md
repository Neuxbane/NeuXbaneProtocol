# Transports & Adapters in nxp

`nxp` decouples business logic from network transport implementations through the **Adapter Architecture**.

> **Non-negotiable principle 6:** Adding a transport = adding an adapter. Never modify `runtime`, `router`, `ipc`, `worker`, or `contract` to add a transport.

---

## 1. The `transport.Adapter` Interface

Every transport implements:

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

### Methods:
- `Name()`: Returns the canonical `abi.Transport` enum value (e.g. `abi.TransportREST`).
- `MatchKey()`: Returns the pattern for dispatch lookup (e.g. `rest:GET:/path` or `mqtt:sensors/+/telemetry`).
- `Normalize(raw any)`: Ingests a protocol-native connection/message (e.g. `*http.Request` or MQTT packet) and maps it into a transport-neutral `*abi.Request`.
- `Render(w io.Writer, resp *abi.Response, ctx RenderCtx)`: Formats the generic `*abi.Response` into protocol-specific wire bytes with protocol features (ETag, Range, QoS acks, status codes).
- `Serve(listener *net.Listener, table *router.Table)`: Runs the protocol listening loop.
- `Shutdown(ctx context.Context)`: Gracefully stops the listener and closes active connections.

---

## 2. Implemented Transports

The framework ships with 10 production adapters under `internal/transport/`:

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

---

## 3. Conformance Invariant

Every transport adapter must pass the test suite in `internal/transport/conformance/conformance_test.go`:
- Normalization produces valid headers, path, query, and payload.
- Rendering respects status codes and response headers.
- Dispatcher cleanly propagates responses and errors.
- Adapters self-register via `init()`:
  ```go
  func init() {
      transport.RegisterAdapter(NewAdapter(nil))
  }
  ```
