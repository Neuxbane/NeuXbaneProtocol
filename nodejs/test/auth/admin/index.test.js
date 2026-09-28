import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/admin (define/auth/admin/index.js)', async (t) => {
  await t.test('GET /auth/admin rejects regular member with 403', async () => {
    const cookie = await getAuthCookie('member_user', 'member');
    const res = await fetch(`${BASE_URL}/auth/admin`, {
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 403);
    const data = await res.json();
    assert.match(data.error, /403/);
  });

  await t.test('GET /auth/admin allows admin user', async () => {
    const cookie = await getAuthCookie('admin_user', 'admin');
    const res = await fetch(`${BASE_URL}/auth/admin`, {
      headers: { Cookie: cookie }
    });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.match(data.message, /Admin control center/i);
    assert.equal(data.admin, 'admin_user');
  });

  await t.test('GET /auth/admin?nxp includes guard info', async () => {
    const res = await fetch(`${BASE_URL}/auth/admin?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'admin_scope_root');
    assert.equal(data.enforcesGuardHere, true);
    assert.ok(Array.isArray(data.inheritedGuards));
  });
});
