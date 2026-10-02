import Ajv from 'ajv';

const ajv = new Ajv({
  allErrors: true,
  coerceTypes: false,
  useDefaults: true,
  strict: false
});

/**
 * Runtime mode. `dev` validates outgoing responses against their declared
 * `schema.response`; `prod` skips that check entirely for zero overhead.
 * Defaults to `dev` so a missing/unknown value never silently disables checks.
 */
export const NXP_MODE = (process.env.NXP_MODE || 'dev').toLowerCase() === 'prod' ? 'prod' : 'dev';
export const isDev = NXP_MODE === 'dev';
export const isProd = NXP_MODE === 'prod';

export class NxpNode {
  /**
   * @param {Object} options
   * @param {string} options.name - Action or Module name
   * @param {string} options.description - Semantic description for LLMs and humans
   * @param {string} [options.method='ANY'] - HTTP/WS Method constraint ('GET', 'POST', 'ANY', 'WS')
   * @param {Object} [options.schema={}] - Contract for the node. Either a bare
   *   request schema (shorthand or JSON Schema) or a `{ request, response }`
   *   pair. `request` validates incoming params; `response` validates the
   *   outgoing body in dev mode.
   * @param {Object} [options.messageSchema=null] - WebSocket incoming message JSON Schema
   * @param {Object} [options.messageResponseSchema=null] - WebSocket outgoing message contract
   * @param {boolean} [options.websocket=false] - Whether this node accepts WebSocket upgrades
   * @param {Object} [options.ws=null] - WebSocket lifecycle handlers: { open, message, close, error }
   * @param {Function} [options.wsHandler=null] - Direct WebSocket connection handler
   * @param {Function} [options.guard] - Middleware interceptor (only active if node is an index.js)
   * @param {Function} [options.handler] - Terminal endpoint logic
   */
  constructor({
    name,
    description,
    method = 'ANY',
    schema = {},
    messageSchema = null,
    messageResponseSchema = null,
    websocket = false,
    ws = null,
    wsHandler = null,
    guard = null,
    handler = null,
    streamBody = false
  }) {
    this.name = name;
    this.description = description;
    this.isWebSocket = Boolean(
      websocket ||
      ws ||
      wsHandler ||
      (typeof method === 'string' && method.toUpperCase() === 'WS')
    );
    this.method = this.isWebSocket && method === 'ANY' && !handler ? 'WS' : method.toUpperCase();

    // `schema` may be a bare request contract or a `{ request, response }` pair.
    const { request, response } = splitSchema(schema);
    this.schema = request;
    this.responseSchema = response;

    // Capture `file`-typed fields BEFORE normalization rewrites `type: 'file'`
    // into a JSON-Schema-valid form (normalizeSchema mutates raw JSON Schema in
    // place, so collecting afterwards would miss the marker).
    this.requestFileFields = collectFileFields(request);
    this.responseFileFields = collectFileFields(response);

    this.validator = ajv.compile(normalizeFileTypes(normalizeSchema(structuredClone(request))));
    this.responseValidator = response
      ? ajv.compile(normalizeFileTypes(normalizeSchema(structuredClone(response))))
      : null;

    this.messageSchema = messageSchema;
    this.messageValidator = messageSchema ? ajv.compile(normalizeSchema(messageSchema)) : null;
    this.messageResponseSchema = messageResponseSchema;
    this.messageResponseValidator = messageResponseSchema
      ? ajv.compile(normalizeSchema(messageResponseSchema))
      : null;
    this.ws = ws;
    this.wsHandler = wsHandler;
    this.guard = guard;
    this.handler = handler;
    this.streamBody = Boolean(streamBody);
    this.isScopeRoot = false;
    this.children = new Map();
    this.clients = new Set();
  }

  mount(segment, childNode) {
    this.children.set(segment, childNode);
    return this;
  }

  validateParams(params) {
    if (!this.validator(params)) {
      const details = this.validator.errors
        .map((error) => `${error.instancePath || '$'} ${error.message}`)
        .join('; ');
      throw new Error(`400: ${details}.`);
    }

    return params;
  }

  validateMessage(data) {
    if (!this.messageValidator) return data;
    if (!this.messageValidator(data)) {
      const details = this.messageValidator.errors
        .map((error) => `${error.instancePath || '$'} ${error.message}`)
        .join('; ');
      throw new Error(`400: ${details}.`);
    }

    return data;
  }

  /**
   * Validate an outgoing HTTP response body against `responseSchema`.
   * Only enforced in dev mode; in prod this is a no-op so there is no
   * runtime cost and no chance of breaking a live response.
   *
   * @param {*} data - The value the handler returned (pre-serialization).
   * @returns {*} The same value, unchanged.
   * @throws {Error} `500: ...` with the failing paths when the contract is broken.
   */
  validateResponse(data) {
    if (!isDev || !this.responseValidator) return data;
    if (!this.responseValidator(data)) {
      throw new Error(`500: Response schema violation at '${this.name}': ${formatErrors(this.responseValidator.errors)}`);
    }
    return data;
  }

  /**
   * Validate an outgoing WebSocket message against `messageResponseSchema`.
   * Only enforced in dev mode. Returns the value unchanged so it can be
   * passed straight to `send`/`broadcast`.
   *
   * @param {*} data - The value being sent to a client.
   * @returns {*} The same value, unchanged.
   * @throws {Error} When the outgoing message breaks the contract.
   */
  validateMessageResponse(data) {
    if (!isDev || !this.messageResponseValidator) return data;
    if (!this.messageResponseValidator(data)) {
      throw new Error(`Response schema violation at '${this.name}' (ws): ${formatErrors(this.messageResponseValidator.errors)}`);
    }
    return data;
  }

  broadcast(data, { exclude = null } = {}) {
    const payload = typeof data === 'string' ? data : JSON.stringify(data);
    for (const client of this.clients) {
      if (client !== exclude && client.readyState === 1 /* WebSocket.OPEN */) {
        client.send(payload);
      }
    }
  }

  handleWebSocket(ws, baseCtx) {
    this.clients.add(ws);

    const send = (data) => {
      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        const payload = typeof data === 'string' ? data : JSON.stringify(data);
        ws.send(payload);
      }
    };

    const broadcast = (data, options = {}) => {
      this.broadcast(data, { exclude: options.includeSelf ? null : ws, ...options });
    };

    const wsCtx = {
      ...baseCtx,
      ws,
      send,
      broadcast,
      clients: this.clients
    };

    const wsConfig = this.ws || {};
    const openHandler = wsConfig.open || wsConfig.onOpen || wsConfig.connect || wsConfig.onConnect;
    const messageHandler = wsConfig.message || wsConfig.onMessage;
    const closeHandler = wsConfig.close || wsConfig.onClose;
    const errorHandler = wsConfig.error || wsConfig.onError;

    if (typeof openHandler === 'function') {
      try {
        openHandler(wsCtx);
      } catch (err) {
        if (typeof errorHandler === 'function') {
          errorHandler(wsCtx, err);
        } else {
          console.error(`WebSocket open error on '${baseCtx.pathname}':`, err);
        }
      }
    }

    if (typeof this.wsHandler === 'function') {
      try {
        this.wsHandler(wsCtx);
      } catch (err) {
        if (typeof errorHandler === 'function') {
          errorHandler(wsCtx, err);
        } else {
          console.error(`WebSocket wsHandler error on '${baseCtx.pathname}':`, err);
        }
      }
    }

    ws.on('message', async (raw, isBinary) => {
      try {
        let message = raw;
        if (!isBinary) {
          const str = raw.toString('utf8');
          try {
            message = JSON.parse(str);
          } catch {
            message = str;
          }
        }

        if (this.messageValidator) {
          message = this.validateMessage(message);
        }

        if (typeof messageHandler === 'function') {
          await messageHandler(wsCtx, message, { raw, isBinary });
        }
      } catch (err) {
        if (typeof errorHandler === 'function') {
          errorHandler(wsCtx, err);
        } else {
          send({ error: err.message });
        }
      }
    });

    ws.on('close', (code, reason) => {
      this.clients.delete(ws);
      if (typeof closeHandler === 'function') {
        try {
          closeHandler(wsCtx, code, reason ? reason.toString('utf8') : '');
        } catch (err) {
          console.error(`WebSocket close error on '${baseCtx.pathname}':`, err);
        }
      }
    });

    ws.on('error', (err) => {
      if (typeof errorHandler === 'function') {
        errorHandler(wsCtx, err);
      } else {
        console.error(`WebSocket socket error on '${baseCtx.pathname}':`, err);
      }
    });
  }

  isLeaf() {
    return this.children.size === 0;
  }

  resolvePipeline(segments, currentPath = '', params = {}) {
    const pipeline = [{ node: this, path: currentPath || '/', params: { ...params } }];
    if (segments.length === 0) return pipeline;

    const [head, ...tail] = segments;

    // Literal match first, then a dynamic `[param]` child (e.g. `[id]`).
    let nextNode = this.children.get(head);
    let nextParams = params;
    if (!nextNode) {
      for (const [key, child] of this.children.entries()) {
        const match = /^\[(.+)\]$/.exec(key);
        if (match) {
          nextNode = child;
          nextParams = { ...params, [match[1]]: decodeURIComponent(head) };
          break;
        }
      }
    }
    if (!nextNode) return null;

    const nextPipeline = nextNode.resolvePipeline(tail, `${currentPath}/${head}`, nextParams);
    if (!nextPipeline) return null;

    return pipeline.concat(nextPipeline);
  }

  describe(currentPath = '/', activeParentGuards = []) {
    const effectiveGuards = [...activeParentGuards];
    if (this.isScopeRoot && typeof this.guard === 'function') {
      effectiveGuards.push(currentPath);
    }

    const manifest = {
      path: currentPath,
      name: this.name,
      description: this.description,
      method: this.method,
      mode: NXP_MODE,
      isWebSocket: this.isWebSocket,
      isScopeRoot: this.isScopeRoot,
      enforcesGuardHere: this.isScopeRoot && typeof this.guard === 'function',
      inheritedGuards: effectiveGuards,
      isLeaf: this.isLeaf(),
      schema: {
        request: this.schema,
        response: this.responseSchema || null
      },
      messageSchema: this.messageSchema || null,
      messageResponseSchema: this.messageResponseSchema || null,
      children: {}
    };

    for (const [key, child] of this.children.entries()) {
      const childPath = currentPath === '/' ? `/${key}` : `${currentPath}/${key}`;
      manifest.children[key] = child.describe(childPath, effectiveGuards);
    }

    return manifest;
  }
}

/**
 * Render Ajv errors into a compact, human-readable string.
 */
function formatErrors(errors = []) {
  return errors
    .map((error) => `${error.instancePath || '$'} ${error.message}`)
    .join('; ');
}

/**
 * Split a node `schema` into its request and response contracts.
 *
 * Accepts either a bare request schema (shorthand field map or JSON Schema)
 * or a `{ request, response }` pair. The pair form is detected only when the
 * object has exactly the `request`/`response` keys, so a shorthand field
 * literally named `request` or `response` is still treated as a field.
 */
function splitSchema(schema) {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) {
    return { request: {}, response: null };
  }

  const keys = Object.keys(schema);
  const isPair =
    keys.length > 0 &&
    keys.every((key) => key === 'request' || key === 'response') &&
    (schema.request !== undefined || schema.response !== undefined);

  if (isPair) {
    return {
      request: schema.request ?? {},
      response: schema.response ?? null
    };
  }

  return { request: schema, response: null };
}

function normalizeSchema(schema) {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return {};

  // Raw JSON Schema is passed through (with the friendly `limit` alias applied).
  if (isJsonSchema(schema)) return normalizeArrayLimits(schema);

  const properties = {};
  const required = [];
  for (const [key, rule] of Object.entries(schema)) {
    const { required: isRequired, ...normalizedRule } = rule || {};
    properties[key] = normalizedRule;
    if (isRequired) required.push(key);
  }

  return normalizeArrayLimits({ type: 'object', properties, required });
}

/**
 * Convert `file`-typed schema nodes into a JSON-Schema-valid form for Ajv.
 *
 * Ajv has no `file` type, so a `{ type: 'file' }` node is rewritten to accept
 * either a string (a signed link token or upload id) or an object with an `id`
 * (a resolved record). The original `type: 'file'` marker is preserved on a
 * sibling `nxpFile` flag so the runtime can resolve/serialize it.
 */
function normalizeFileTypes(schema) {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return schema;

  if (schema.type === 'file') {
    schema.nxpFile = true;
    schema.type = ['string', 'object'];
    // The `file()` helper may set a boolean `required` for the shorthand field
    // map; in raw JSON Schema `required` must be an array at the object level,
    // so drop the boolean to keep the compiled schema valid.
    if (typeof schema.required === 'boolean') delete schema.required;
  }

  if (schema.properties && typeof schema.properties === 'object') {
    for (const value of Object.values(schema.properties)) normalizeFileTypes(value);
  }
  if (schema.items && typeof schema.items === 'object' && !Array.isArray(schema.items)) {
    normalizeFileTypes(schema.items);
  } else if (Array.isArray(schema.items)) {
    schema.items.forEach(normalizeFileTypes);
  }
  for (const keyword of ['allOf', 'anyOf', 'oneOf']) {
    if (Array.isArray(schema[keyword])) schema[keyword].forEach(normalizeFileTypes);
  }

  return schema;
}

/**
 * Distinguish a raw JSON Schema document from the shorthand field map.
 * Only unambiguous JSON Schema keywords count, so a shorthand field that
 * happens to be named `items`, `type`, `required`, etc. is still treated
 * as a field rather than a schema.
 */
function isJsonSchema(schema) {
  return (
    typeof schema.type === 'string' ||
    typeof schema.$ref === 'string' ||
    (schema.properties && typeof schema.properties === 'object') ||
    (schema.$defs && typeof schema.$defs === 'object') ||
    (schema.definitions && typeof schema.definitions === 'object') ||
    Array.isArray(schema.required) ||
    Array.isArray(schema.allOf) ||
    Array.isArray(schema.anyOf) ||
    Array.isArray(schema.oneOf) ||
    typeof schema.additionalProperties === 'boolean' ||
    (schema.not && typeof schema.not === 'object')
  );
}

/**
 * Accept a friendly `limit` on array schemas as an alias for `maxItems`.
 * Applies recursively to array items and object properties.
 */
function normalizeArrayLimits(schema) {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return schema;

  if (schema.type === 'array' && schema.limit != null && schema.maxItems == null) {
    schema.maxItems = schema.limit;
  }
  delete schema.limit;

  if (schema.items && typeof schema.items === 'object' && !Array.isArray(schema.items)) {
    normalizeArrayLimits(schema.items);
  } else if (Array.isArray(schema.items)) {
    schema.items.forEach(normalizeArrayLimits);
  }

  if (schema.properties && typeof schema.properties === 'object') {
    for (const value of Object.values(schema.properties)) {
      normalizeArrayLimits(value);
    }
  }

  for (const keyword of ['allOf', 'anyOf', 'oneOf']) {
    if (Array.isArray(schema[keyword])) schema[keyword].forEach(normalizeArrayLimits);
  }

  return schema;
}

/**
 * Build an array schema with an optional length cap.
 * @param {Object} items - Schema for each array entry.
 * @param {Object} [options]
 * @param {number} [options.minItems] - Minimum number of entries.
 * @param {number} [options.maxItems] - Maximum number of entries (omitted = unlimited).
 * @param {number} [options.limit] - Alias for maxItems.
 */
export function arrayOf(items, { minItems, maxItems, limit } = {}) {
  const schema = { type: 'array', items };
  const cap = maxItems ?? limit;
  if (minItems != null) schema.minItems = minItems;
  if (cap != null) schema.maxItems = cap;
  return schema;
}

/**
 * Declare a file field in a request or response schema.
 *
 * In a **request** schema, the runtime resolves the incoming reference (a
 * signed link token or an upload id) to a stored file record and passes it to
 * the handler as `ctx.params.<field> = { id, originalName, size, ... }`. The
 * handler never touches multipart or chunk mechanics.
 *
 * In a **response** schema, the handler returns `{ id }` (or a full record) and
 * the runtime serializes it to `{ id, url, expiresAt }`, where `url` is a
 * stateless signed link (`/__nxp/link/<token>`).
 *
 * @param {Object} [options]
 * @param {boolean} [options.required] - Whether the field must be present.
 * @param {string} [options.description] - Human/LLM-facing description.
 */
export function file({ required = false, description } = {}) {
  const schema = { type: 'file' };
  if (required) schema.required = true;
  if (description) schema.description = description;
  return schema;
}

/**
 * True when a normalized schema node declares the `file` type.
 */
export function isFileSchema(schema) {
  return Boolean(schema && typeof schema === 'object' && schema.type === 'file');
}

/**
 * Collect the top-level field names whose schema declares `type: 'file'`.
 * Handles both shorthand field maps and raw JSON Schema `properties`.
 *
 * @param {Object} schema - The authored request or response schema.
 * @returns {string[]} Field names that carry a file reference.
 */
export function collectFileFields(schema) {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return [];

  const fields = [];
  const inspect = (node) => {
    if (!node || typeof node !== 'object') return;
    if (node.type === 'file') return true;
    return false;
  };

  // Raw JSON Schema form.
  if (schema.properties && typeof schema.properties === 'object') {
    for (const [key, value] of Object.entries(schema.properties)) {
      if (inspect(value)) fields.push(key);
    }
    return fields;
  }

  // Shorthand field map form.
  for (const [key, value] of Object.entries(schema)) {
    if (inspect(value)) fields.push(key);
  }
  return fields;
}
