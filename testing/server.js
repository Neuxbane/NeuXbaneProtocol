import http from 'node:http';
import https from 'node:https';
import fs from 'node:fs/promises';
import { createReadStream, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORT = Number(process.env.TESTING_PORT || process.env.PORT) || 4000;

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon'
};

function setCorsHeaders(res) {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD');
  res.setHeader('Access-Control-Allow-Headers', '*');
  res.setHeader('Access-Control-Expose-Headers', '*');
}

const server = http.createServer(async (req, res) => {
  const parsedUrl = new URL(req.url, `http://${req.headers.host || 'localhost'}`);
  const pathname = parsedUrl.pathname;

  // Handle CORS preflight for all routes
  if (req.method === 'OPTIONS') {
    setCorsHeaders(res);
    res.writeHead(204);
    return res.end();
  }

  // Transparent API Proxy
  // Allows testing endpoints on different hosts/ports without CORS restrictions,
  // and allows the frontend to inspect cookies, upload files, and stream downloads.
  if (pathname === '/api/proxy' || pathname.startsWith('/api/proxy/')) {
    setCorsHeaders(res);

    const targetUrlStr = parsedUrl.searchParams.get('url') || req.headers['x-target-url'];
    if (!targetUrlStr) {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      return res.end(JSON.stringify({ error: 'Missing target URL parameter (?url=... or X-Target-Url header)' }));
    }

    let targetUrl;
    try {
      targetUrl = new URL(targetUrlStr);
    } catch {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      return res.end(JSON.stringify({ error: `Invalid target URL: '${targetUrlStr}'` }));
    }

    const isHttps = targetUrl.protocol === 'https:';
    const client = isHttps ? https : http;

    const proxyHeaders = { ...req.headers };
    proxyHeaders.host = targetUrl.host;

    // If client supplied a custom cookie header via x-proxy-cookie, apply it to cookie header
    if (req.headers['x-proxy-cookie']) {
      proxyHeaders.cookie = req.headers['x-proxy-cookie'];
    }

    delete proxyHeaders['x-target-url'];
    delete proxyHeaders['x-proxy-cookie'];

    const options = {
      method: req.method,
      headers: proxyHeaders
    };

    const proxyReq = client.request(targetUrl, options, (proxyRes) => {
      setCorsHeaders(res);

      // Expose Set-Cookie headers as X-Proxy-Set-Cookie so browser JS can read and store them in IndexedDB
      const setCookie = proxyRes.headers['set-cookie'];
      if (setCookie) {
        res.setHeader('x-proxy-set-cookie', Array.isArray(setCookie) ? setCookie.join('; ') : setCookie);
      }

      // Forward target headers
      for (const [key, value] of Object.entries(proxyRes.headers)) {
        if (key.toLowerCase() !== 'transfer-encoding') {
          res.setHeader(key, value);
        }
      }

      res.writeHead(proxyRes.statusCode || 200);
      proxyRes.pipe(res);
    });

    proxyReq.on('error', (err) => {
      setCorsHeaders(res);
      if (!res.headersSent) {
        res.writeHead(502, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          error: `Proxy failed to connect to ${targetUrl.origin}: ${err.message}`,
          code: err.code || 'PROXY_ERROR'
        }));
      }
    });

    // Stream client request body directly into proxy request (supports large uploads & multipart)
    return req.pipe(proxyReq);
  }

  // Health check endpoint
  if (pathname === '/api/health') {
    setCorsHeaders(res);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    return res.end(JSON.stringify({ status: 'ok', service: 'nxp-testing-server', port: PORT }));
  }

  // Static File Serving
  let filePath = path.join(__dirname, pathname === '/' ? 'index.html' : pathname);

  if (!existsSync(filePath)) {
    // Single-page application fallback
    filePath = path.join(__dirname, 'index.html');
  }

  try {
    const ext = path.extname(filePath).toLowerCase();
    const contentType = MIME_TYPES[ext] || 'application/octet-stream';
    res.writeHead(200, { 'Content-Type': contentType });
    createReadStream(filePath).pipe(res);
  } catch (err) {
    res.writeHead(500, { 'Content-Type': 'text/plain' });
    res.end(`Error loading ${pathname}: ${err.message}`);
  }
});

server.listen(PORT, () => {
  console.log('============================================================');
  console.log(` NeuXbane Protocol (NXP) Test Explorer Server`);
  console.log(` Running on: http://localhost:${PORT}`);
  console.log(` Open this link in your browser to test NXP endpoints.`);
  console.log('============================================================');
});

export { server };
