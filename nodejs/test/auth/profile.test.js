import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/profile (define/auth/profile.js)', async (t) => {
  await t.test('GET /auth/profile requires auth guard (401)', async () => {
    const res = await fetch(`${BASE_URL}/auth/profile`);
    assert.equal(res.status, 401);
  });

  await t.test('GET /auth/profile returns profile for member', async () => {
    const cookie = await getAuthCookie('bob', 'member');
    const res = await fetch(`${BASE_URL}/auth/profile`, {
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.profile.username, 'bob');
    assert.equal(data.profile.role, 'member');
    assert.equal(data.status, 'Active');
  });

  await t.test('GET /auth/profile?nxp describes endpoint', async () => {
    const res = await fetch(`${BASE_URL}/auth/profile?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'user_profile');
    assert.equal(data.method, 'GET');
    assert.ok(Array.isArray(data.inheritedGuards));
  });
});
