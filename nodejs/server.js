import http from 'node:http';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath, pathToFileURL } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DEFINE_DIR = path.join(__dirname, 'define');
const SESSION_SECRET = crypto.randomBytes(32).toString('hex');
const SESSIONS = new Map();

function getOrCreateSession(req, res) {
  const cookieHeader = req.headers.cookie || '';
  const match = cookieHeader.match(/(?:^|;\s*)nxp_sid=([^;]+)/);
  let sid = match ? match[1] : null;

  if (sid) {
    const [rawId, signature] = sid.split('.');
    const expected = crypto.createHmac('sha256', SESSION_SECRET).update(rawId).digest('hex');
    if (signature !== expected || !SESSIONS.has(rawId)) {
      sid = null;
    } else {
      sid = rawId;
    }
  }

  if (!sid) {
    sid = crypto.randomUUID();
    const signature = crypto.createHmac('sha256', SESSION_SECRET).update(sid).digest('hex');
    SESSIONS.set(sid, new Map());
    res.setHeader('Set-Cookie', `nxp_sid=${sid}.${signature}; Path=/; HttpOnly; SameSite=Lax`);
  }

  const store = SESSIONS.get(sid);
  return {
    id: sid,
    get: (key) => store.get(key),
    set: (key, value) => store.set(key, value),
    delete: (key) => store.delete(key),
    clear: () => store.clear()
  };
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
  res.setHeader('Content-Type', 'application/json');
  const session = getOrCreateSession(req, res);

  let body = {};
  if (['POST', 'PUT', 'PATCH'].includes(req.method)) {
    const buffers = [];
    for await (const chunk of req) buffers.push(chunk);
    const raw = Buffer.concat(buffers).toString();
    if (raw) {
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

  const ctx = { req, res, params, session, targetNode, pathname };

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

    res.writeHead(200);
    res.end(JSON.stringify(result));
  } catch (error) {
    let status = 500;
    if (error.message.startsWith('401')) status = 401;
    else if (error.message.startsWith('403')) status = 403;

    res.writeHead(status);
    res.end(JSON.stringify({ error: error.message }));
  }
});

server.listen(3000, () => {
  console.log('Neuxbane Protocol Server running on http://localhost:3000');
});
