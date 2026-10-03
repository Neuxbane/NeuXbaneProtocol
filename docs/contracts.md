# Contracts & Schema Enforcement in nxp

In `nxp`, **types are the contract**. Handlers declare strongly-typed input and result structs. The framework reflects Go struct tags into **JSON Schema Draft 2020-12** at compile time and enforces validation in the Mother process.

---

## 1. Struct Tags & Schema Reflection

`nxp/abi.SchemaOf[T]()` inspects struct fields and tags to construct schema definitions:

| Tag | Purpose | Example |
| :--- | :--- | :--- |
| `json` | Serialized field name and omission rules | `json:"user_id,omitempty"` |
| `validate` | Validation constraints (`required`, `min=X`, `max=Y`, `email`, `uuid`) | `validate:"required,min=1"` |
| `format` | JSON Schema semantic format | `format:"email"`, `format:"date-time"` |
| `doc` | Schema field documentation | `doc:"Unique identifier for the item"` |

### Example Handler Types
```go
type CreateItemPayload struct {
    Name  string `json:"name"  validate:"required"`
    Price int    `json:"price" validate:"min=1"`
}

type CreateItemResult struct {
    ID    string `json:"id"    validate:"required"`
    Name  string `json:"name"  validate:"required"`
    Price int    `json:"price" validate:"min=1"`
}
```

---

## 2. Polymorphic Route Shapes

Every route defines its contract using one of five concrete implementations of `abi.Shape`:

1. `RequestResponseShape`: Unary request/response (REST, gRPC).
   - `Request *abi.Schema`
   - `Responses map[int]*abi.Schema`
2. `FramesShape`: Bidirectional streams of message frames (WebSocket, WebRTC DataChannels).
   - `In *abi.Schema`
   - `Out *abi.Schema`
3. `DatagramShape`: Lossy or fast datagram transmissions (UDP).
   - `Body *abi.Schema`
   - `MaxSize int`
   - `Response DatagramResponse` (`none`, `echo`, `unicast`)
4. `PubSubShape`: Topic-routed message broker streams (MQTT, AMQP, Redis Pub/Sub).
   - `Topics map[string]string`
   - `QoS int`
   - `Retain bool`
   - `In, Out *abi.Schema`
5. `StreamShape`: Partitioned durable event log streams (Kafka, NATS JetStream, Redis Streams).
   - `Partitions int`
   - `ConsumerGroup string`
   - `In, Out *abi.Schema`

---

## 3. Mother Enforcement Policy

### Principle: The mother enforces, the worker trusts
- **Ingress Validation:** Before a request is forwarded to an IPC worker, Mother validates query params, headers, and payload against the route's `Shape`. If invalid, an error response is returned immediately (`contract.request.invalid`), and the worker process is never invoked.
- **Egress Validation:** After a worker completes execution, Mother validates the result payload and status code.

### Egress Violation Policies:
Configured via `egress_policy`:
- `warn`: Logs contract discrepancy and emits metrics; passes response to client.
- `quarantine` (Default): Logs critical violation, marks worker as suspicious, replaces response with a safe sanitized error (`contract.response.invalid`).
- `kill`: Immediately terminates the offending worker process with SIGKILL and restarts a fresh worker instance.

---

## 4. Structured Violations & Error Codes

Contract errors return stable codes and JSON Pointer paths:

| Error Code | HTTP Status | Description |
| :--- | :--- | :--- |
| `contract.request.invalid` | 400 | Inbound payload or parameter failed schema validation |
| `contract.request.unauthorized`| 401 | Missing or invalid authentication token |
| `contract.request.forbidden` | 403 | Authenticated identity lacks required scope |
| `contract.request.ratelimited` | 429 | Rate limit exceeded |
| `contract.response.invalid` | 502 | Worker produced payload violating route schema |
| `contract.worker.unavailable` | 503 | Worker socket disconnected or crashed |

---

## 5. Schema Bundles

The full schema definition for the active build is queryable via:
```bash
curl "http://localhost:8080/__nxp/schema?nxp&build=<build-id>"
```
Response header:
```http
Content-Type: application/schema+json; charset=utf-8
Cache-Control: public, max-age=31536000, immutable
```
