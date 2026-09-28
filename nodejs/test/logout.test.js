import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie } from './helper.js';

await ensureServerRunning();

test('Endpoint: /logout (define/logout.js)', async (t) => {
  await t.test('POST /logout clears session cookie', async () => {
    const cookie = await getAuthCookie('alice', 'member');
    const res = await fetch(`${BASE_URL}/logout`, {
      method: 'POST',
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.success, true);
    const setCookie = res.headers.get('set-cookie');
    assert.ok(setCookie && setCookie.includes('Max-Age=0'));
  });

  await t.test('GET /logout?nxp returns manifest', async () => {
    const res = await fetch(`${BASE_URL}/logout?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'session_logout');
    assert.equal(data.method, 'POST');
  });
});
