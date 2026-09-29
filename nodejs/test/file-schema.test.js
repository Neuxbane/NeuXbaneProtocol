import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from './helper.js';

await ensureServerRunning();

const UPLOAD_URL = `${BASE_URL}/__nxp/upload`;

async function initSession(cookie, body) {
  const res = await fetch(UPLOAD_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body)
  });
  return { res, data: await res.json() };
}

async function putChunk(cookie, uploadId, index, bytes) {
  const res = await fetch(`${UPLOAD_URL}/${uploadId}/chunk/${index}`, {
    method: 'PUT',
    headers: { Cookie: cookie },
    body: bytes
  });
  return { res, data: await res.json() };
}

/** Upload a small file end-to-end and return its stored record. */
async function storeFile(cookie, name, payload) {
  const { data: session } = await initSession(cookie, {
    originalName: name,
    size: payload.length,
    chunkSize: payload.length,
    mimeType: 'application/octet-stream'
  });
  await putChunk(cookie, session.uploadId, 0, Buffer.from(payload));
  const res = await fetch(`${UPLOAD_URL}/${session.uploadId}/complete`, {
    method: 'POST',
    headers: { Cookie: cookie }
  });
  const { file } = await res.json();
  return file;
}

test('Files as schema fields (request resolve + response serialize)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('a response `file` field is serialized to a signed link', async () => {
      const file = await storeFile(cookie, 'res.bin', 'hello-response');
      const res = await fetch(`${BASE_URL}/__nxp/link-test?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.link.id, file.id);
      assert.match(data.link.url, /^\/__nxp\/link\//);
      assert.ok(Date.parse(data.link.expiresAt) > Date.now());
    });

    await t.test('a request `file` field resolves a file id to a record', async () => {
      const file = await storeFile(cookie, 'req.bin', 'hello-request');
      const res = await fetch(`${BASE_URL}/__nxp/describe-file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ ref: file.id })
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.id, file.id);
      assert.equal(data.originalName, 'req.bin');
      assert.equal(data.size, 'hello-request'.length);
      assert.equal(data.checksum, file.checksum);
    });

    await t.test('a request `file` field also accepts a signed link token', async () => {
      const file = await storeFile(cookie, 'req-link.bin', 'via-token');
      const linkRes = await fetch(`${BASE_URL}/__nxp/link-test?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      const { link } = await linkRes.json();
      const token = link.url.split('/').pop();

      const res = await fetch(`${BASE_URL}/__nxp/describe-file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ ref: token })
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.id, file.id);
      assert.equal(data.originalName, 'req-link.bin');
    });

    await t.test('an unknown file reference is rejected (404)', async () => {
      const res = await fetch(`${BASE_URL}/__nxp/describe-file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ ref: '00000000-0000-0000-0000-000000000000' })
      });
      assert.equal(res.status, 404);
    });

    await t.test('a malformed file reference is rejected (400)', async () => {
      const res = await fetch(`${BASE_URL}/__nxp/describe-file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ ref: 'not-a-uuid' })
      });
      assert.equal(res.status, 400);
    });

    await t.test('a missing required `file` field is rejected (400)', async () => {
      const res = await fetch(`${BASE_URL}/__nxp/describe-file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({})
      });
      assert.equal(res.status, 400);
    });
  });
});
