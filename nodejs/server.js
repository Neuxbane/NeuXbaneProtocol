import http from 'node:http';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { WebSocketServer } from 'ws';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DEFINE_DIR = path.join(__dirname, 'define');
const SESSION_SECRET = process.env.NXP_SESSION_SECRET || crypto.randomBytes(32).toString('hex');
const SESSION_KEY = crypto.createHash('sha256').update(SESSION_SECRET).digest();
const SESSION_COOKIE = 'nxp_session';
const SESSION_MAX_AGE = 60 * 60 * 24 * 7;

function encryptSession(session) {
  const iv = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv('aes-256-gcm', SESSION_KEY, iv);
  const encrypted = Buffer.concat([
    cipher.update(JSON.stringify(session), 'utf8'),
    cipher.final()
  ]);
  const tag = cipher.getAuthTag();

  return [iv, tag, encrypted]
    .map((part) => part.toString('base64url'))
    .join('.');
}

function decryptSession(value) {
  const [encodedIv, encodedTag, encodedData] = value.split('.');
  if (!encodedIv || !encodedTag || !encodedData) return null;

  try {
    const decipher = crypto.createDecipheriv(
      'aes-256-gcm',
      SESSION_KEY,
      Buffer.from(encodedIv, 'base64url')
    );
    decipher.setAuthTag(Buffer.from(encodedTag, 'base64url'));
    const decrypted = Buffer.concat([
      decipher.update(Buffer.from(encodedData, 'base64url')),
      decipher.final()
    ]);
    const session = JSON.parse(decrypted.toString('utf8'));

    return session && typeof session === 'object' ? session : null;
  } catch {
    return null;
  }
}

function getSessionState(req) {
  const cookieHeader = req.headers.cookie || '';
  const match = cookieHeader.match(new RegExp(`(?:^|;\\s*)${SESSION_COOKIE}=([^;]+)`));
  const stored = match ? decryptSession(match[1]) : null;
  return stored || { id: crypto.randomUUID(), data: {} };
}

function getSessionFromRequest(req) {
  const state = getSessionState(req);
  return {
    id: state.id,
    get: (key) => state.data[key],
    set: (key, value) => {
      state.data[key] = value;
    },
    delete: (key) => delete state.data[key],
    clear: () => {
      state.data = {};
    }
  };
}

function getOrCreateSession(req, res) {
  const state = getSessionState(req);
  let cleared = false;

  const session = {
    id: state.id,
    get: (key) => state.data[key],
    set: (key, value) => {
      state.data[key] = value;
      cleared = false;
    },
    delete: (key) => delete state.data[key],
    clear: () => {
      state.data = {};
      cleared = true;
    }
  };

  const originalWriteHead = res.writeHead.bind(res);
  res.writeHead = (...args) => {
    if (cleared) {
      res.setHeader(
        'Set-Cookie',
        `${SESSION_COOKIE}=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0`
      );
    } else {
      const value = encryptSession(state);
      res.setHeader(
        'Set-Cookie',
        `${SESSION_COOKIE}=${value}; Path=/; HttpOnly; SameSite=Lax; Max-Age=${SESSION_MAX_AGE}`
      );
    }

    return originalWriteHead(...args);
  };

  return session;
}

async function buildTree(dirPath) {
  let rootNode;
  const entries = await fs.readdir(dirPath, { withFileTypes: true });
  const indexFile = entries.find((entry) => entry.isFile() && entry.name === 'index.js');

  if (indexFile) {
    const mod = await import(pathToFileURL(path.join(dirPath, indexFile.name)));
    rootNode = mod.default;
    rootNode.isScopeRoot = true;
  }

  for (const entry of entries) {
    if (entry.name === 'index.js') continue;
    const fullPath = path.join(dirPath, entry.name);

    if (entry.isDirectory()) {
      const subTree = await buildTree(fullPath);
      if (subTree && rootNode) rootNode.mount(entry.name, subTree);
    } else if (entry.isFile() && entry.name.endsWith('.js')) {
      const segment = entry.name.replace(/\.js$/, '');
      const mod = await import(pathToFileURL(fullPath));
      mod.default.isScopeRoot = false;
      if (rootNode) rootNode.mount(segment, mod.default);
    }
  }

  return rootNode;
}

const rootNode = await buildTree(DEFINE_DIR);

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`);
  const pathname = url.pathname.replace(/\/+$/, '') || '/';
  const contentType = (req.headers['content-type'] || '').toLowerCase();
  const isMultipart = contentType.startsWith('multipart/form-data');
  res.setHeader('Content-Type', 'application/json');
  const session = getOrCreateSession(req, res);

  let rawBody = Buffer.alloc(0);
  let body = {};
  if (['POST', 'PUT', 'PATCH'].includes(req.method)) {
    const buffers = [];
    for await (const chunk of req) buffers.push(chunk);
    rawBody = Buffer.concat(buffers);

    // Multipart payloads are parsed by the handler (e.g. uploadService), not here.
    if (!isMultipart && rawBody.length) {
      const raw = rawBody.toString('utf8');
      try {
        body = JSON.parse(raw);
      } catch {
        body = Object.fromEntries(new URLSearchParams(raw));
      }
    }
  }

  const queryParams = Object.fromEntries(url.searchParams.entries());
  const params = { ...queryParams, ...body };
  const segments = pathname === '/' ? [] : pathname.slice(1).split('/').filter(Boolean);
  const pipeline = rootNode.resolvePipeline(segments);

  if (!pipeline) {
    res.writeHead(404);
    return res.end(JSON.stringify({ error: `Route '${pathname}' not found in NXP tree.` }));
  }

  const targetStep = pipeline[pipeline.length - 1];
  const targetNode = targetStep.node;

  if (url.searchParams.has('nxp')) {
    res.writeHead(200);
    return res.end(JSON.stringify(targetNode.describe(targetStep.path), null, 2));
  }

  if (targetNode.isWebSocket && !targetNode.handler) {
    res.writeHead(426, {
      'Upgrade': 'websocket',
      'Connection': 'Upgrade'
    });
    return res.end(JSON.stringify({
      error: `Route '${pathname}' is a WebSocket endpoint. Connect using WebSocket (ws:// or wss://).`
    }));
  }

  let validatedParams;
  try {
    validatedParams = targetNode.validateParams(params);
  } catch (error) {
    res.writeHead(400);
    return res.end(JSON.stringify({ error: error.message }));
  }

  const ctx = {
    req,
    res,
    params: validatedParams,
    query: queryParams,
    rawBody,
    session,
    targetNode,
    pathname,
    setHeader: (name, value) => res.setHeader(name, value),
    respond: (status, data) => {
      if (res.writableEnded) return;
      if (!res.headersSent) res.writeHead(status);
      if (typeof data === 'string' || Buffer.isBuffer(data)) {
        res.end(data);
      } else {
        res.end(JSON.stringify(data));
      }
    }
  };

  try {
    for (const step of pipeline) {
      if (step.node.isScopeRoot && typeof step.node.guard === 'function') {
        await step.node.guard(ctx);
      }
    }

    if (targetNode.method !== 'ANY' && targetNode.method !== req.method) {
      res.writeHead(405);
      return res.end(JSON.stringify({
        error: `Method ${req.method} not allowed. Expects ${targetNode.method}`
      }));
    }

    const result = targetNode.handler
      ? await targetNode.handler(ctx)
      : { message: `Node at '${pathname}' ready.`, children: Array.from(targetNode.children.keys()) };

    // A handler may take over the response (streaming, binary, custom status).
    if (res.writableEnded || res.headersSent) return;

    res.writeHead(200);
    res.end(JSON.stringify(result));
  } catch (error) {
    // Guards and handlers may throw "NNN: message" to signal an HTTP status.
    const statusMatch = /^(\d{3}):/.exec(error.message || '');
    const status = statusMatch ? Number(statusMatch[1]) : 500;

    if (res.writableEnded || res.headersSent) return;

    res.writeHead(status);
    res.end(JSON.stringify({ error: error.message }));
  }
});

const wss = new WebSocketServer({ noServer: true });

server.on('upgrade', async (req, socket, head) => {
  try {
    const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`);
    const pathname = url.pathname.replace(/\/+$/, '') || '/';
    const segments = pathname === '/' ? [] : pathname.slice(1).split('/').filter(Boolean);
    const pipeline = rootNode.resolvePipeline(segments);

    if (!pipeline) {
      socket.write('HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nRoute not found\r\n');
      socket.destroy();
      return;
    }

    const targetStep = pipeline[pipeline.length - 1];
    const targetNode = targetStep.node;

    if (!targetNode.isWebSocket) {
      socket.write('HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nEndpoint is not a WebSocket route\r\n');
      socket.destroy();
      return;
    }

    const session = getSessionFromRequest(req);
    const queryParams = Object.fromEntries(url.searchParams.entries());

    let validatedParams = queryParams;
    try {
      validatedParams = targetNode.validateParams(queryParams);
    } catch (error) {
      socket.write(`HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\n${error.message}\r\n`);
      socket.destroy();
      return;
    }

    const ctx = { req, params: validatedParams, session, targetNode, pathname };

    for (const step of pipeline) {
      if (step.node.isScopeRoot && typeof step.node.guard === 'function') {
        await step.node.guard(ctx);
      }
    }

    wss.handleUpgrade(req, socket, head, (ws) => {
      targetNode.handleWebSocket(ws, ctx);
    });
  } catch (error) {
    let status = 'HTTP/1.1 500 Internal Server Error';
    if (error.message?.startsWith('401')) status = 'HTTP/1.1 401 Unauthorized';
    else if (error.message?.startsWith('403')) status = 'HTTP/1.1 403 Forbidden';

    socket.write(`${status}\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\n${error.message}\r\n`);
    socket.destroy();
  }
});

const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`Neuxbane Protocol Server running on http://localhost:${PORT}`);
});

export { server, wss, rootNode, buildTree, encryptSession, decryptSession, getSessionFromRequest, getSessionState };
