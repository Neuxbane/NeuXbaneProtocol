import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning } from './helper.js';

await ensureServerRunning();

test('Endpoint: / (define/index.js)', async (t) => {
  await t.test('GET / returns global entrypoint and route directory', async () => {
    const res = await fetch(`${BASE_URL}/`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.match(data.message, /Neuxbane Protocol Online/);
    assert.ok(Array.isArray(data.routes));
    assert.ok(data.routes.includes('counter'));
    assert.ok(data.routes.includes('login'));
    assert.ok(data.routes.includes('ws'));
  });

  await t.test('GET /?nxp returns root manifest', async () => {
    const res = await fetch(`${BASE_URL}/?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'root_portal');
    assert.equal(data.path, '/');
    assert.ok(data.children);
  });
});
