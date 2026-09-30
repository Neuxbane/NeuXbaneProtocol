# NeuXbane Protocol or NXP

NeuXbane Protocol or NXP is an example core for a self-describing, filesystem-defined API runtime. It is designed for both conventional clients and AI agents that need to discover available operations, understand their contracts, and execute them through HTTP.

The example implementation is in `nodejs/`.

## Core Ideas

- The `define/` directory becomes the API tree.
- Each `NxpNode` describes an operation or scope (HTTP or WebSocket).
- `index.js` files define scope roots and can provide guards.
- Child scopes inherit through the resolved request pipeline.
- Ajv validates request parameters and WebSocket messages using JSON Schema.
- Nodes declare a `schema: { request, response }` contract. In dev mode the runtime also validates outgoing responses against `schema.response` and throws a detailed `500` on drift; prod mode skips the check.
- `?nxp` exposes machine-readable route metadata.
- Session state is stored in an encrypted, stateless HTTP-only cookie.
- Full WebSocket support with scope guards, session access, message schemas, and broadcasting.
- Runtime background services (e.g. background counter) with real-time WebSocket live-streaming and HTTP control endpoints.
- Dedicated file uploads: `multipart/form-data` parsed with `busboy` and stored on disk with JSON metadata.
- Files as first-class resources: declare a `file` field and the runtime resolves requests and emits stateless signed download links — handlers never touch upload mechanics.
- Dynamic path parameters via `[name]` folders/files, captured into `ctx.params`.
- Array inputs with an optional length cap (`arrayOf(..., { maxItems })` or a friendly `limit` alias).
- Mirrored test suite in `test/` that tests against the running server.

## Directory Structure

```text
nodejs/
├── NxpNode.js
├── server.js
├── services/
│   ├── counterService.js     # Background runtime state manager
│   ├── uploadService.js      # Multipart parsing, disk storage, resumable chunked sessions
│   ├── fileLink.js           # Stateless HMAC-signed file links (no server state)
│   ├── streamFile.js         # Shared Range-capable file streaming
│   └── formService.js        # Bounded/unbounded multi-item collections
├── define/
│   ├── index.js              # /
│   ├── login.js              # POST /login
│   ├── logout.js             # POST /logout
│   ├── ws.js                 # WS /ws (public real-time stream)
│   ├── auth/
│   │   ├── index.js          # /auth authentication guard
│   │   ├── live.js           # WS /auth/live (protected real-time stream)
│   │   ├── profile.js        # GET /auth/profile
│   │   ├── upload/
│   │   │   ├── index.js      # GET /auth/upload (upload hub)
│   │   │   ├── file.js       # POST /auth/upload/file (multipart upload)
│   │   │   ├── list.js       # GET /auth/upload/list
│   │   │   ├── download.js   # GET /auth/upload/download
│   │   │   └── delete.js     # POST /auth/upload/delete
│   │   ├── form/
│   │   │   ├── index.js      # GET /auth/form (multi-item hub)
│   │   │   ├── add.js        # POST /auth/form/add (bounded array)
│   │   │   ├── unbounded.js  # POST /auth/form/unbounded (unlimited array)
│   │   │   └── clear.js      # POST /auth/form/clear
│   │   └── admin/
│   │       ├── index.js      # /auth/admin admin guard
│   │       └── users.js      # GET /auth/admin/users
│   ├── counter/
│   │   ├── index.js          # GET /counter & WS /counter (live tick stream)
│   │   ├── start.js          # POST /counter/start
│   │   ├── stop.js           # POST /counter/stop
│   │   └── reset.js          # POST /counter/reset
│   └── __nxp/                # Reserved runtime namespace, defined in the tree
│       ├── index.js          # /__nxp (public capability root)
│       ├── describe-file.js  # POST /__nxp/describe-file (request `file` example)
│       ├── link-test.js      # GET /__nxp/link-test (response `file` example)
│       ├── upload/
│       │   ├── index.js      # POST /__nxp/upload (init) & GET (list)
│       │   └── [id]/
│       │       ├── index.js  # GET /__nxp/upload/:id (status)
│       │       ├── chunk/[index].js  # PUT .../chunk/:index (streamed)
│       │       ├── complete.js        # POST .../complete
│       │       └── abort.js           # DELETE .../abort
│       ├── download/[id].js  # GET /__nxp/download/:id (gated, Range-capable)
│       └── link/[token].js   # GET /__nxp/link/:token (redeem signed link)
└── test/                     # Mirrored test suite
    ├── helper.js             # Server liveness check, auth & lock helpers
    ├── index.test.js
    ├── login.test.js
    ├── logout.test.js
    ├── ws.test.js
    ├── upload-chunk.test.js  # Resumable chunked upload + Range download
    ├── file-schema.test.js   # `file` request resolve + response serialization
    ├── __nxp/
    │   ├── link.test.js      # Stateless signed link redemption
    │   └── download.test.js  # Gated download + HTTP Range
    ├── auth/
    │   ├── index.test.js
    │   ├── live.test.js
    │   ├── profile.test.js
    │   ├── upload/
    │   │   ├── index.test.js
    │   │   ├── file.test.js
    │   │   ├── list.test.js
    │   │   ├── download.test.js
    │   │   └── delete.test.js
    │   ├── form/
    │   │   ├── index.test.js
    │   │   ├── add.test.js
    │   │   ├── unbounded.test.js
    │   │   └── clear.test.js
    │   └── admin/
    │       ├── index.test.js
    │       └── users.test.js
    └── counter/
        ├── index.test.js
        ├── start.test.js
        ├── stop.test.js
        └── reset.test.js
```

## Requirements

- Node.js 18 or newer
- npm

## Run It

```bash
cd nodejs
npm install
NXP_SESSION_SECRET="replace-this-with-a-long-secret" npm run dev
```

The server listens on:

```text
http://localhost:3000
```

### Development vs Production Mode

The runtime has two modes, selected with the `NXP_MODE` environment variable (or the matching npm script):

| Script | Mode | Response validation |
| --- | --- | --- |
| `npm run dev` | `dev` | **On** — every outgoing response is checked against its `schema.response` contract. A violation returns `500` with the failing paths. |
| `npm run prod` | `prod` | **Off** — response validation is skipped entirely for zero overhead. |
| `npm start` | `dev` (default) | Same as `npm run dev`. |

```bash
# Development: validate responses, surface contract drift immediately
npm run dev

# Production: skip response validation
npm run prod
```

In dev mode, if a handler returns a body that does not match its declared `schema.response`, the server responds with `500` and a message such as:

```json
{ "error": "500: Response schema violation at 'session_login': /success must be boolean" }
```

This is the runtime equivalent of a TypeScript type error for your API responses: the developer declares the contract, and the framework enforces it while developing. In production the check is removed, so a schema mistake can never take down live traffic.

## Running Tests

The test suite tests live endpoints against an active server. It automatically checks if the server is running before executing; if the server is not running, it outputs a friendly notice and exits without running tests.

Tests **require dev mode** so response schemas are enforced. If the server is running in `prod` mode, the suite refuses to start and tells you to restart with `npm run dev`.

1. Start the server in one terminal:
```bash
cd nodejs
npm run dev
```

2. Run the tests in another terminal:
```bash
cd nodejs
npm test
```

For local development, `NXP_SESSION_SECRET` is optional. The server generates a temporary secret when it is missing, which invalidates existing sessions after a restart. Use a stable secret outside development.

## Try the API

Login and save the returned cookie:

```bash
curl -i -c cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","role":"admin"}' \
  http://localhost:3000/login
```

Read the authenticated profile:

```bash
curl -i -b cookies.txt http://localhost:3000/auth/profile
```

Read the admin-only directory:

```bash
curl -i -b cookies.txt http://localhost:3000/auth/admin/users
```

Log out:

```bash
curl -i -b cookies.txt -c cookies.txt -X POST http://localhost:3000/logout
```

A regular member can log in by omitting the role or setting it to `member`. That user can access `/auth/profile` but receives `403 Forbidden` from `/auth/admin/users`.

## File Uploads

Uploads live behind the `/auth` guard. Send files as `multipart/form-data` using the `file` field; any other text field (for example `tags` or `folder`) is stored as metadata. Multiple files can be sent in a single request.

```bash
# Upload a file with metadata
curl -b cookies.txt \
  -F "file=@./README.md" \
  -F "tags=demo, docs" \
  -F "folder=notes" \
  http://localhost:3000/auth/upload/file

# List stored files
curl -b cookies.txt http://localhost:3000/auth/upload/list

# Download the original bytes
curl -b cookies.txt -OJ "http://localhost:3000/auth/upload/download?id=<file-id>"

# Delete a file
curl -b cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"id":"<file-id>"}' \
  http://localhost:3000/auth/upload/delete
```

Files are **content-addressed**: the payload is stored once under `nodejs/uploads/blobs/<sha256>` and every upload record references that shared blob. Uploading identical bytes again reuses the existing blob instead of writing a duplicate, so storage does not grow with duplicates. Each upload gets its own `<id>.meta.json` sidecar with a sanitized original name, size, MIME type, and the `sha256` checksum. A blob is only deleted once no metadata record references it; orphaned blobs (e.g. from abandoned uploads) are garbage-collected by the stale sweeper. Configuration environment variables:

- `NXP_UPLOAD_DIR` - storage directory (defaults to `nodejs/uploads/`)
- `NXP_MAX_FILE_SIZE` - per-file cap in bytes (defaults to 10 MB, returns `413`)
- `NXP_MAX_FILES` - files per request (defaults to 10, returns `413`)
- `NXP_BLOB_GC_GRACE` - grace period (ms) before an unreferenced blob is collectable (defaults to 1 hour)

## Files as Resources

A file is a first-class resource: a route **declares** a field as a file and the runtime handles the bytes. Handlers never see multipart bodies, chunk state, upload ids, or link URLs — they just read a resolved record or return an `{ id }` reference.

Import the `file` helper from `NxpNode.js`:

```js
import { NxpNode, file } from '../../NxpNode.js';

export default new NxpNode({
  name: 'describe_file',
  method: 'POST',
  schema: {
    request: {
      type: 'object',
      properties: { ref: file({ required: true }) },
      required: ['ref'],
      additionalProperties: false
    },
    response: {
      type: 'object',
      properties: { link: file() },
      required: ['link'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const record = ctx.params.ref;      // already resolved to a stored record
    return { link: { id: record.id } }; // runtime turns this into a signed link
  }
});
```

**Request `file` field** — the client sends either a **signed link token** or a bare **file id**. Before the handler runs, the runtime resolves it to the stored record (see `uploadService.resolveRef`). A bad id yields `400`/`404`.

**Response `file` field** — the handler returns `{ id }` (or a full record) and the runtime serializes it to:

```json
{ "id": "<file-id>", "url": "/__nxp/link/<token>", "expiresAt": "2026-01-01T00:00:00.000Z" }
```

The `url` is a **stateless, cryptographically signed link**. The token is `base64url(payload).base64url(HMAC-SHA256(payload, key))` where the key is derived from `NXP_SESSION_SECRET` with a domain separator (`:filelink`), so a file link can never be confused with a session cookie. The payload carries its own `exp` — the server stores **no per-request state**, exactly like the encrypted session cookie. Anyone holding a valid, unexpired token can fetch the file; a tampered, malformed, or expired token returns `403`.

Two example routes ship under `define/__nxp/` to demonstrate the contract:

- `POST /__nxp/describe-file` with `{ "ref": "<id-or-token>" }` — the request `file` field resolved to a record.
- `GET /__nxp/link-test?id=<file-id>` — returns `{ link: { id, url, expiresAt } }` from a response `file` field.

### Gated Downloads

`GET /__nxp/download/:id` streams a stored file and applies an **example access policy** (`uploadService.canAccess`) before serving: a file is readable when it is marked `public`, when the requester is the recorded owner (`uploadedBy`), or when the requester is an `admin`. Otherwise the request is rejected with `403`. This is deliberately example logic — a real deployment would consult a database. The same policy helper is where you would slot in transfer-of-ownership checks.

```bash
# Download by id (login/owner required unless the file is public)
curl -b cookies.txt http://localhost:3000/__nxp/download/<file-id>

# Redeem a signed link — NO cookie required; the token is the capability
curl http://localhost:3000/__nxp/link/<token>
```

Serving uses the shared `streamFile` helper, so `/__nxp/download` is Range-capable too (see [Resumable Downloads](#resumable-downloads)).

Configuration environment variables:

- `NXP_LINK_TTL` - signed-link lifetime in milliseconds (defaults to 15 minutes)

## Resumable Chunked Uploads

For large files, the single-shot `multipart/form-data` endpoint is not ideal: a dropped connection means starting over. The runtime exposes a resumable, chunked upload protocol under the `/__nxp/upload` namespace. Unlike earlier releases, `__nxp` is **defined in the `define/` tree** (`define/__nxp/`) like any other scope, so it uses the same routing, guards, and schema validation — there is no hardcoded interception in `server.js`.

The flow is: **init** a session, **PUT** chunks (any order, parallel-safe, idempotent), then **complete** to assemble the final file. If the connection drops, re-`GET` the session status to learn which chunks are missing and resume from there.

```bash
# 1. Init a session — returns an uploadId and the chunk plan
curl -b cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"originalName":"big.iso","size":15728640,"mimeType":"application/octet-stream"}' \
  http://localhost:3000/__nxp/upload

# 2. Upload chunks (index is 0-based). Repeat for each index; order does not matter.
curl -b cookies.txt -X PUT \
  --data-binary @chunk-0.bin \
  http://localhost:3000/__nxp/upload/<uploadId>/chunk/0

# 3. Check status — reports received/missing chunks so you can resume
curl -b cookies.txt http://localhost:3000/__nxp/upload/<uploadId>

# 4. Complete — assembles chunks in order, verifies the checksum, stores the file
curl -b cookies.txt -X POST http://localhost:3000/__nxp/upload/<uploadId>/complete

# Abort and discard a session
curl -b cookies.txt -X DELETE http://localhost:3000/__nxp/upload/<uploadId>/abort

# List active sessions
curl -b cookies.txt http://localhost:3000/__nxp/upload
```

Endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/__nxp/upload` | Create a session; returns `uploadId`, `chunkSize`, `totalChunks` |
| `GET` | `/__nxp/upload` | List active sessions |
| `GET` | `/__nxp/upload/:id` | Session status: `received[]`, `missing[]`, `complete` |
| `PUT` | `/__nxp/upload/:id/chunk/:index` | Upload one chunk (streamed to disk; `400` on bad index/length) |
| `POST` | `/__nxp/upload/:id/complete` | Assemble + verify; `409` if chunks are missing |
| `DELETE` | `/__nxp/upload/:id/abort` | Abort and discard the session |

Chunks are stored as indexed parts under `uploads/.chunks/<uploadId>/<index>.part`, so out-of-order and parallel uploads are safe and re-uploading a chunk is idempotent. Completing concatenates the parts in index order, computes the `sha256` checksum, publishes the payload to the shared content-addressed blob store (reusing an existing blob when the bytes already exist), writes the same `<id>.meta.json` sidecar as a normal upload, and removes the session directory. Sessions older than the TTL are swept automatically.

Configuration environment variables:

- `NXP_CHUNK_SIZE` - chunk size in bytes (defaults to 5 MB)
- `NXP_CHUNK_TTL` - session time-to-live in milliseconds (defaults to 24 hours)
- `NXP_BLOB_GC_GRACE` - grace period (ms) before an unreferenced blob is collectable (defaults to 1 hour)

### Resumable Downloads

`GET /auth/upload/download` supports HTTP `Range` requests, so clients can resume an interrupted download or fetch a slice of a large file. It advertises `Accept-Ranges: bytes`, answers a satisfiable range with `206 Partial Content` and a `Content-Range` header, and returns `416 Range Not Satisfiable` for an unsatisfiable range.

```bash
# Fetch bytes 0-1023
curl -b cookies.txt -r 0-1023 "http://localhost:3000/auth/upload/download?id=<file-id>"

# Fetch the last 512 bytes (suffix range)
curl -b cookies.txt -r -512 "http://localhost:3000/auth/upload/download?id=<file-id>"
```

## Multi-Item Forms

The `/auth/form` scope appends arrays of entries to a collection. `/auth/form/add` is bounded by `maxItems` (default 5, override with `NXP_FORM_LIMIT`), while `/auth/form/unbounded` accepts any number.

```bash
# Bounded: rejects requests that exceed the limit with 400
curl -b cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"name":"Ada"},"Grace",42]}' \
  http://localhost:3000/auth/form/add

# Unbounded: no maxItems
curl -b cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"items":["a","b","c","d","e","f","g"]}' \
  http://localhost:3000/auth/form/unbounded

# Clear either collection
curl -b cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"target":"bounded"}' \
  http://localhost:3000/auth/form/clear
```

## Route Discovery

Add `?nxp` to inspect a route definition:

```bash
curl http://localhost:3000/login?nxp
```

The response includes the route path, operation name, description, HTTP method, `schema` (with `request` and `response`), scope information, and child nodes. This metadata is intended to help humans, tools, and AI clients understand the API without separate route documentation.

## Defining a Node

A route is an exported `NxpNode` instance:

```js
import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'create_note',
  description: 'Creates a note for the current user.',
  method: 'POST',
  schema: {
    request: {
      type: 'object',
      properties: {
        title: { type: 'string', minLength: 1 },
        labels: {
          type: 'array',
          items: { type: 'string' },
          uniqueItems: true
        }
      },
      required: ['title'],
      additionalProperties: false
    },
    response: {
      type: 'object',
      properties: {
        title: { type: 'string' },
        labels: { type: 'array', items: { type: 'string' } }
      },
      required: ['title', 'labels'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    title: ctx.params.title,
    labels: ctx.params.labels || []
  })
});
```

Handlers receive a context object containing:

- `ctx.params` - validated request parameters
- `ctx.query` - raw query parameters (even when overridden in the body)
- `ctx.rawBody` - the raw request body as a `Buffer` (used for `multipart/form-data`)
- `ctx.session` - session access
- `ctx.req` - Node.js request
- `ctx.res` - Node.js response
- `ctx.targetNode` - resolved node
- `ctx.pathname` - requested path
- `ctx.setHeader(name, value)` - set a response header
- `ctx.respond(status, data)` - take over the response (JSON, string, or `Buffer`)

A handler that writes to `ctx.res` itself (streaming, binary, or a custom status) skips the default JSON response.

### Dynamic Path Parameters

A folder or file named `[name]` matches any segment value and captures it into `ctx.params.name`:

```text
define/
  notes/
    [id].js        # GET /notes/:id  ->  ctx.params.id
```

Values are URI-decoded before capture. Literal children take precedence over dynamic ones, so a fixed route (`/notes/latest`) wins over `/notes/:id` when both exist. This is what the `/__nxp` download, link, and upload routes use for their `:id`, `:index`, and `:token` segments.

## Schema Support

Schemas use JSON Schema through Ajv. The runtime supports nested objects, arrays, defaults, references, reusable definitions, composition, and standard constraints.

A node's `schema` describes both directions of the contract:

```js
schema: {
  request: { /* validates incoming params (HTTP) or handshake (WS) */ },
  response: { /* validates the outgoing body in dev mode */ }
}
```

- `schema.request` is validated on every request. Invalid input returns HTTP `400` with details.
- `schema.response` is validated against the handler's return value **in dev mode only**. A mismatch returns HTTP `500` with the failing paths. In prod mode it is ignored.

Both halves accept either raw JSON Schema or the shorthand field map. A bare `schema` (without `request`/`response` keys) is treated as the request contract, so existing nodes keep working unchanged:

```js
// Bare form: shorthand request schema, no response contract.
schema: {
  username: { type: 'string', required: true },
  role: { type: 'string', enum: ['member', 'admin'], default: 'member' }
}
```

The pair form is only detected when the object's keys are exactly `request` and/or `response`, so a shorthand field literally named `request` or `response` is still treated as a field.

Reusable definitions can be declared with `$defs` and referenced with `$ref`:

```js
schema: {
  request: {
    type: 'object',
    properties: {
      user: { $ref: '#/$defs/user' }
    },
    required: ['user'],
    $defs: {
      user: {
        type: 'object',
        properties: {
          username: { type: 'string', minLength: 1 },
          age: { type: 'integer', minimum: 0, default: 18 }
        },
        required: ['username'],
        additionalProperties: false
      }
    }
  }
}
```

The original shorthand format remains supported:

```js
schema: {
  username: { type: 'string', required: true },
  role: { type: 'string', enum: ['member', 'admin'], default: 'member' }
}
```

A shorthand field that is literally named `type`, `required`, or `items` is still treated as a field; raw JSON Schema is only detected when an unambiguous schema keyword (like a string `type`, `properties`, or `$ref`) is present.

### Array Inputs and Limits

Accept an array of entries with `arrayOf()`, and cap its length with `maxItems` (or the shorter `limit` alias). Omitting the cap makes the array unbounded.

```js
import { NxpNode, arrayOf } from '../NxpNode.js';

// Capped at 5 entries.
schema: {
  type: 'object',
  properties: {
    items: arrayOf(
      { oneOf: [{ type: 'object' }, { type: 'string' }, { type: 'number' }] },
      { minItems: 1, maxItems: 5 }
    )
  },
  required: ['items'],
  additionalProperties: false
}
```

```js
// No cap: any number of entries is accepted.
schema: {
  type: 'object',
  properties: { items: arrayOf({ type: 'string' }, { minItems: 1 }) }
}
```

`limit` is normalized to `maxItems` anywhere in a schema, so `{ type: 'array', items: {...}, limit: 5 }` works too. Ajv enforces both `minItems` and `maxItems`; the included `/auth/form/add` (bounded) and `/auth/form/unbounded` routes demonstrate each mode.

Invalid input returns HTTP `400` with validation details.

### File Fields

Declare a field as a file with the `file()` helper. In a **request** schema the runtime resolves the incoming reference (a signed token or file id) to a stored record; in a **response** schema it serializes the handler's `{ id }` into a signed link. The handler never touches upload mechanics. See [Files as Resources](#files-as-resources) for the full contract.

```js
import { NxpNode, file } from '../NxpNode.js';

schema: {
  request: {
    type: 'object',
    properties: { ref: file({ required: true }) },
    required: ['ref'],
    additionalProperties: false
  }
}
```

## Sessions

The example uses an encrypted stateless cookie named `nxp_session`. The session API is available to handlers:

```js
ctx.session.get('user');
ctx.session.set('key', value);
ctx.session.delete('key');
ctx.session.clear();
```

The cookie uses AES-256-GCM. Set a stable `NXP_SESSION_SECRET` so sessions survive server restarts. For production deployments, use HTTPS and add the appropriate `Secure` cookie behavior, CSRF protection, expiration policy, and session revocation strategy.

## Current Scope

This repository is an example core and is not yet a complete production server. Before deploying it publicly, add or configure:

- automated tests
- request body size limits and timeouts
- strict content-type handling
- structured logging and request IDs
- rate limiting
- graceful shutdown and health checks
- explicit authorization metadata in route manifests
- response schemas and structured error schemas
- production session hardening and CSRF protection

## Project Status

The project is exploring an agent-friendly API model where routing, validation, authorization boundaries, and machine-readable descriptions are represented by the same node tree.
