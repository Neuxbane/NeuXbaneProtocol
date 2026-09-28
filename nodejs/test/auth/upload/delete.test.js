import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

async function uploadFile(cookie, name, contents) {
  const form = new FormData();
  form.append('file', new Blob([contents], { type: 'text/plain' }), name);
  const res = await fetch(`${BASE_URL}/auth/upload/file`, {
    method: 'POST',
    headers: { Cookie: cookie },
    body: form
  });
  return (await res.json()).files[0];
}

test('Endpoint: /auth/upload/delete (define/auth/upload/delete.js)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('POST /auth/upload/delete requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: '00000000-0000-0000-0000-000000000000' })
      });
      assert.equal(res.status, 401);
    });

    await t.test('POST /auth/upload/delete removes the file and its metadata', async () => {
      const uploaded = await uploadFile(cookie, 'delete-me.txt', 'bye');

      const res = await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: uploaded.id })
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.id, uploaded.id);

      const after = await fetch(`${BASE_URL}/auth/upload/download?id=${uploaded.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(after.status, 404);
    });

    await t.test('POST /auth/upload/delete returns 404 for an unknown id', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: '99999999-8888-7777-6666-555555555555' })
      });
      assert.equal(res.status, 404);
    });

    await t.test('POST /auth/upload/delete validates required id (400)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({})
      });
      assert.equal(res.status, 400);
    });

    await t.test('POST /auth/upload/delete?nxp returns endpoint manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/delete?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'delete_upload');
      assert.equal(data.method, 'POST');
    });
  });
});
