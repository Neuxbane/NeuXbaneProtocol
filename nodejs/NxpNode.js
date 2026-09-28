import Ajv from 'ajv';

const ajv = new Ajv({
  allErrors: true,
  coerceTypes: false,
  useDefaults: true,
  strict: false
});

export class NxpNode {
  /**
   * @param {Object} options
   * @param {string} options.name - Action or Module name
   * @param {string} options.description - Semantic description for LLMs and humans
   * @param {string} [options.method='ANY'] - HTTP/WS Method constraint ('GET', 'POST', 'ANY', 'WS')
   * @param {Object} [options.schema={}] - Parameter/handshake contract for AI tools
   * @param {Object} [options.messageSchema=null] - WebSocket incoming message JSON Schema
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
    websocket = false,
    ws = null,
    wsHandler = null,
    guard = null,
    handler = null
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
    this.schema = schema;
    this.validator = ajv.compile(normalizeSchema(schema));
    this.messageSchema = messageSchema;
    this.messageValidator = messageSchema ? ajv.compile(normalizeSchema(messageSchema)) : null;
    this.ws = ws;
    this.wsHandler = wsHandler;
    this.guard = guard;
    this.handler = handler;
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

  resolvePipeline(segments, currentPath = '') {
    const pipeline = [{ node: this, path: currentPath || '/' }];
    if (segments.length === 0) return pipeline;

    const [head, ...tail] = segments;
    const nextNode = this.children.get(head);
    if (!nextNode) return null;

    const nextPipeline = nextNode.resolvePipeline(tail, `${currentPath}/${head}`);
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
      isWebSocket: this.isWebSocket,
      isScopeRoot: this.isScopeRoot,
      enforcesGuardHere: this.isScopeRoot && typeof this.guard === 'function',
      inheritedGuards: effectiveGuards,
      isLeaf: this.isLeaf(),
      schema: this.schema,
      messageSchema: this.messageSchema || null,
      children: {}
    };

    for (const [key, child] of this.children.entries()) {
      const childPath = currentPath === '/' ? `/${key}` : `${currentPath}/${key}`;
      manifest.children[key] = child.describe(childPath, effectiveGuards);
    }

    return manifest;
  }
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
