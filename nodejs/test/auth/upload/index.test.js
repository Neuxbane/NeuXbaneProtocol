import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/upload (define/auth/upload/index.js)', async (t) => {
  await withLock('upload', async () => {
    await t.test('GET /auth/upload requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload`);
      assert.equal(res.status, 401);
      const data = await res.json();
      assert.match(data.error, /401/);
    });

    await t.test('GET /auth/upload returns file hub state', async () => {
      const cookie = await getAuthCookie('alice', 'admin');
      const res = await fetch(`${BASE_URL}/auth/upload`, { headers: { Cookie: cookie } });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.user.username, 'alice');
      assert.ok(data.uploads);
      assert.equal(typeof data.uploads.count, 'number');
      assert.ok(Array.isArray(data.children));
      assert.ok(data.children.includes('file'));
      assert.ok(data.children.includes('list'));
    });

    await t.test('GET /auth/upload?nxp returns scope manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'upload_scope_root');
      assert.equal(data.isScopeRoot, true);
      assert.ok(Array.isArray(data.inheritedGuards));
      assert.ok(data.children.file);
    });
  });
});
