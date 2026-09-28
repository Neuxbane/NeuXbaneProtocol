import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const PORT = process.env.PORT || 3000;
export const BASE_URL = process.env.TEST_URL || `http://localhost:${PORT}`;
export const WS_URL = BASE_URL.replace(/^http/, 'ws');

let checked = false;

/**
 * Serialize tests that mutate shared server state (a background service, the
 * uploads directory, a form collection, ...) across parallel test files.
 */
export async function withLock(name, fn) {
  const lockFile = path.join(__dirname, `.${name}.lock`);
  const start = Date.now();
  while (true) {
    try {
      const fd = fs.openSync(lockFile, 'wx');
      fs.closeSync(fd);
      break;
    } catch {
      if (Date.now() - start > 20000) {
        try { fs.unlinkSync(lockFile); } catch {}
      }
      await new Promise((r) => setTimeout(r, 40));
    }
  }

  try {
    return await fn();
  } finally {
    try { fs.unlinkSync(lockFile); } catch {}
  }
}

export async function ensureServerRunning() {
  if (checked) return;
  try {
    const res = await fetch(`${BASE_URL}/`, { signal: AbortSignal.timeout(1000) });
    if (res.ok || res.status < 500) {
      checked = true;
      return;
    }
  } catch {
    // server not running
  }

  console.log(`\n======================================================`);
  console.log(`[Neuxbane Protocol Test] Server is NOT running at ${BASE_URL}`);
  console.log(`Tests are configured to verify live endpoints.`);
  console.log(`Skipping tests. Please start the server in another terminal:`);
  console.log(`  cd nodejs && npm start`);
  console.log(`======================================================\n`);
  process.exit(0);
}

export async function getAuthCookie(username = 'alice', role = 'admin') {
  const res = await fetch(`${BASE_URL}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, role })
  });
  return res.headers.get('set-cookie');
}

export async function withCounterLock(fn) {
  return withLock('counter', fn);
}
