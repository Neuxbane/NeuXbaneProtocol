import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/upload/download (define/auth/upload/download.js)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('GET /auth/upload/download requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=00000000-0000-0000-0000-000000000000`);
      assert.equal(res.status, 401);
    });

    await t.test('GET /auth/upload/download returns the stored bytes', async () => {
      const payload = 'download round-trip payload';
      const form = new FormData();
      form.append('file', new Blob([payload], { type: 'text/plain' }), 'roundtrip.txt');
      const uploadRes = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        headers: { Cookie: cookie },
        body: form
      });
      const uploaded = (await uploadRes.json()).files[0];

      const res = await fetch(`${BASE_URL}/auth/upload/download?id=${uploaded.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(res.status, 200);
      assert.match(res.headers.get('content-type'), /text\/plain/);
      assert.match(res.headers.get('content-disposition'), /roundtrip\.txt/);
      assert.equal(await res.text(), payload);

      await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: uploaded.id })
      });
    });

    await t.test('GET /auth/upload/download rejects a malformed id (400)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=not-a-uuid`, {
        headers: { Cookie: cookie }
      });
      assert.equal(res.status, 400);
    });

    await t.test('GET /auth/upload/download returns 404 for an unknown id', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=11111111-2222-3333-4444-555555555555`, {
        headers: { Cookie: cookie }
      });
      assert.equal(res.status, 404);
    });

    await t.test('GET /auth/upload/download?nxp returns endpoint manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'download_upload');
      assert.equal(data.method, 'GET');
      assert.equal(data.schema.request.id.type, 'string');
      assert.equal(data.schema.request.id.required, true);
    });
  });
});
