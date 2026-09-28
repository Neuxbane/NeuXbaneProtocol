import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning } from './helper.js';

await ensureServerRunning();

test('Endpoint: /login (define/login.js)', async (t) => {
  await t.test('POST /login sets session cookie', async () => {
    const res = await fetch(`${BASE_URL}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'alice', role: 'admin' })
    });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.success, true);
    assert.match(data.message, /alice/);
    const cookie = res.headers.get('set-cookie');
    assert.ok(cookie && cookie.includes('nxp_session='));
  });

  await t.test('POST /login validates schema', async () => {
    const res = await fetch(`${BASE_URL}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ role: 'admin' }) // missing required username
    });
    assert.equal(res.status, 400);
    const data = await res.json();
    assert.match(data.error, /username/i);
  });

  await t.test('GET /login?nxp returns login manifest', async () => {
    const res = await fetch(`${BASE_URL}/login?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'session_login');
    assert.equal(data.method, 'POST');
    assert.ok(data.schema);
  });
});
