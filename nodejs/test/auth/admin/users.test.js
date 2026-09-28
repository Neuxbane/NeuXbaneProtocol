import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/admin/users (define/auth/admin/users.js)', async (t) => {
  await t.test('GET /auth/admin/users rejects non-admin with 403', async () => {
    const cookie = await getAuthCookie('charlie', 'member');
    const res = await fetch(`${BASE_URL}/auth/admin/users`, {
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 403);
  });

  await t.test('GET /auth/admin/users allows admin and lists users', async () => {
    const cookie = await getAuthCookie('admin_dan', 'admin');
    const res = await fetch(`${BASE_URL}/auth/admin/users`, {
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.adminViewer, 'admin_dan');
    assert.ok(Array.isArray(data.users));
    assert.ok(data.users.length > 0);
  });

  await t.test('GET /auth/admin/users?nxp returns manifest', async () => {
    const res = await fetch(`${BASE_URL}/auth/admin/users?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'list_admin_users');
    assert.equal(data.method, 'GET');
  });
});
