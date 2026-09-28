# NeuXbane Protocol

NeuXbane Protocol is an example core for a self-describing, filesystem-defined API runtime. It is designed for both conventional clients and AI agents that need to discover available operations, understand their contracts, and execute them through HTTP.

The example implementation is in `nodejs/`.

## Core Ideas

- The `define/` directory becomes the API tree.
- Each `NxpNode` describes an operation or scope.
- `index.js` files define scope roots and can provide guards.
- Child scopes inherit through the resolved request pipeline.
- Ajv validates request parameters using JSON Schema.
- `?nxp` exposes machine-readable route metadata.
- Session state is stored in an encrypted, stateless HTTP-only cookie.

A definition such as:

```text
define/auth/admin/users.js
```

maps to:

```text
GET /auth/admin/users
```

## Example Tree

```text
nodejs/
├── NxpNode.js
├── server.js
└── define/
    ├── index.js              # /
    ├── login.js              # POST /login
    ├── logout.js             # POST /logout
    └── auth/
        ├── index.js          # /auth authentication guard
        ├── profile.js        # GET /auth/profile
        └── admin/
            ├── index.js      # /auth/admin admin guard
            └── users.js      # GET /auth/admin/users
```

## Requirements

- Node.js 18 or newer
- npm

## Run It

```bash
cd nodejs
npm install
NXP_SESSION_SECRET="replace-this-with-a-long-secret" npm start
```

The server listens on:

```text
http://localhost:3000
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

## Route Discovery

Add `?nxp` to inspect a route definition:

```bash
curl http://localhost:3000/login?nxp
```

The response includes the route path, operation name, description, HTTP method, schema, scope information, and child nodes. This metadata is intended to help humans, tools, and AI clients understand the API without separate route documentation.

## Defining a Node

A route is an exported `NxpNode` instance:

```js
import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'create_note',
  description: 'Creates a note for the current user.',
  method: 'POST',
  schema: {
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
  handler: async (ctx) => ({
    title: ctx.params.title,
    labels: ctx.params.labels || []
  })
});
```

Handlers receive a context object containing:

- `ctx.params` - validated request parameters
- `ctx.session` - session access
- `ctx.req` - Node.js request
- `ctx.res` - Node.js response
- `ctx.targetNode` - resolved node
- `ctx.pathname` - requested path

## Schema Support

Schemas use JSON Schema through Ajv. The runtime supports nested objects, arrays, defaults, references, reusable definitions, composition, and standard constraints.

Reusable definitions can be declared with `$defs` and referenced with `$ref`:

```js
schema: {
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
```

The original shorthand format remains supported:

```js
schema: {
  username: { type: 'string', required: true },
  role: { type: 'string', enum: ['member', 'admin'], default: 'member' }
}
```

Invalid input returns HTTP `400` with validation details.

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
