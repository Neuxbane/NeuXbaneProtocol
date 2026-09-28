import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/upload/list (define/auth/upload/list.js)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('GET /auth/upload/list requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/list`);
      assert.equal(res.status, 401);
    });

    await t.test('GET /auth/upload/list returns uploaded file metadata', async () => {
      const form = new FormData();
      form.append('file', new Blob(['list-me'], { type: 'text/plain' }), 'listed.txt');
      const uploadRes = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        headers: { Cookie: cookie },
        body: form
      });
      const uploaded = (await uploadRes.json()).files[0];

      const res = await fetch(`${BASE_URL}/auth/upload/list`, { headers: { Cookie: cookie } });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.ok(Array.isArray(data.files));
      assert.equal(typeof data.count, 'number');
      assert.equal(data.count, data.files.length);
      assert.ok(data.maxFileSize > 0);
      assert.ok(data.files.some((f) => f.id === uploaded.id && f.originalName === 'listed.txt'));

      await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: uploaded.id })
      });
    });

    await t.test('GET /auth/upload/list?nxp returns endpoint manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/list?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'list_uploads');
      assert.equal(data.method, 'GET');
    });
  });
});
