import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/form (define/auth/form/index.js)', async (t) => {
  await withLock('form', async () => {
    await t.test('GET /auth/form requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/form`);
      assert.equal(res.status, 401);
      const data = await res.json();
      assert.match(data.error, /401/);
    });

    await t.test('GET /auth/form returns bounded and unbounded collection state', async () => {
      const cookie = await getAuthCookie('alice', 'member');
      await fetch(`${BASE_URL}/auth/form/clear`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ target: 'bounded' })
      });

      const res = await fetch(`${BASE_URL}/auth/form`, { headers: { Cookie: cookie } });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.user.username, 'alice');
      assert.ok(data.bounded);
      assert.equal(typeof data.bounded.limit, 'number');
      assert.equal(data.bounded.unlimited, false);
      assert.equal(data.bounded.count, 0);
      assert.equal(data.unbounded.unlimited, true);
      assert.equal(data.unbounded.limit, null);
      assert.ok(data.children.includes('add'));
      assert.ok(data.children.includes('unbounded'));
    });

    await t.test('GET /auth/form?nxp returns scope manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/form?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'form_scope_root');
      assert.ok(Array.isArray(data.inheritedGuards));
      assert.ok(data.children.add);
    });
  });
});
