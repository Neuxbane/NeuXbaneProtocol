import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

function multipartForm(fileName, contents, { type = 'text/plain', fields = {} } = {}) {
  const form = new FormData();
  form.append('file', new Blob([contents], { type }), fileName);
  for (const [key, value] of Object.entries(fields)) form.append(key, value);
  return form;
}

test('Endpoint: /auth/upload/file (define/auth/upload/file.js)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('POST /auth/upload/file requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        body: multipartForm('nope.txt', 'denied')
      });
      assert.equal(res.status, 401);
    });

    await t.test('POST /auth/upload/file rejects non-multipart body (415)', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ tags: 'json-not-allowed' })
      });
      assert.equal(res.status, 415);
      const data = await res.json();
      assert.match(data.error, /multipart\/form-data/);
    });

    await t.test('POST /auth/upload/file rejects multipart with no file (400)', async () => {
      const form = new FormData();
      form.append('tags', 'no-file-here');
      const res = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        headers: { Cookie: cookie },
        body: form
      });
      assert.equal(res.status, 400);
      const data = await res.json();
      assert.match(data.error, /No files received/);
    });

    await t.test('POST /auth/upload/file stores a multipart file with metadata', async () => {
      const payload = 'hello neuxbane protocol';
      const res = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        headers: { Cookie: cookie },
        body: multipartForm('greeting.txt', payload, {
          fields: { tags: 'demo, test', folder: 'notes' }
        })
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.files.length, 1);

      const file = data.files[0];
      assert.equal(file.originalName, 'greeting.txt');
      assert.equal(file.size, Buffer.byteLength(payload));
      assert.equal(file.mimeType, 'text/plain');
      assert.match(file.checksum, /^sha256:[0-9a-f]{64}$/);
      assert.equal(file.uploadedBy, 'alice');
      assert.equal(file.folder, 'notes');
      assert.deepEqual(file.tags, ['demo', 'test']);

      // Confirm it is indexed by the list endpoint, then clean up.
      const listRes = await fetch(`${BASE_URL}/auth/upload/list`, { headers: { Cookie: cookie } });
      const listData = await listRes.json();
      assert.ok(listData.files.some((f) => f.id === file.id));

      await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: file.id })
      });
    });

    await t.test('POST /auth/upload/file sanitizes a path-traversal filename', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/file`, {
        method: 'POST',
        headers: { Cookie: cookie },
        body: multipartForm('../../evil.txt', 'nope')
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.files[0].originalName, 'evil.txt');

      await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: data.files[0].id })
      });
    });

    await t.test('GET /auth/upload/file?nxp returns endpoint manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/file?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'upload_file');
      assert.equal(data.method, 'POST');
      assert.ok(data.schema);
    });
  });
});
