import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /auth (define/auth/index.js)', async (t) => {
  await t.test('GET /auth rejects unauthenticated with 401', async () => {
    const res = await fetch(`${BASE_URL}/auth`);
    assert.equal(res.status, 401);
    const data = await res.json();
    assert.match(data.error, /401/);
  });

  await t.test('GET /auth with active session succeeds', async () => {
    const cookie = await getAuthCookie('alice', 'member');
    const res = await fetch(`${BASE_URL}/auth`, {
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.match(data.message, /authenticated hub/i);
    assert.equal(data.user.username, 'alice');
  });

  await t.test('GET /auth?nxp returns scope manifest with guard info', async () => {
    const res = await fetch(`${BASE_URL}/auth?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'auth_scope_root');
    assert.equal(data.isScopeRoot, true);
    assert.equal(data.enforcesGuardHere, true);
  });
});
