import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../helper.js';

await ensureServerRunning();

const UPLOAD_URL = `${BASE_URL}/__nxp/upload`;
const DOWNLOAD_URL = `${BASE_URL}/__nxp/download`;

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

test('Stored file download + gating (/__nxp/download)', async (t) => {
  await withLock('upload', async () => {
    const owner = await getAuthCookie('alice', 'admin');

    await t.test('the owner can download and the bytes match', async () => {
      const payload = 'owner-visible-bytes';
      const file = await storeFile(owner, 'own.bin', payload);

      const res = await fetch(`${DOWNLOAD_URL}/${file.id}`, { headers: { Cookie: owner } });
      assert.equal(res.status, 200);
      assert.equal(res.headers.get('accept-ranges'), 'bytes');
      assert.equal(await res.text(), payload);
    });

    await t.test('an anonymous request to a private file is forbidden (403)', async () => {
      const file = await storeFile(owner, 'private.bin', 'secret');
      const res = await fetch(`${DOWNLOAD_URL}/${file.id}`);
      assert.equal(res.status, 403);
    });

    await t.test('a byte range is served with 206 and Content-Range', async () => {
      const payload = 'abcdefghij';
      const file = await storeFile(owner, 'range.bin', payload);

      const res = await fetch(`${DOWNLOAD_URL}/${file.id}`, {
        headers: { Cookie: owner, Range: 'bytes=2-4' }
      });
      assert.equal(res.status, 206);
      assert.equal(res.headers.get('content-range'), `bytes 2-4/${payload.length}`);
      assert.equal(await res.text(), 'cde');
    });

    await t.test('an unsatisfiable range is rejected (416)', async () => {
      const file = await storeFile(owner, 'bad-range.bin', 'abc');
      const res = await fetch(`${DOWNLOAD_URL}/${file.id}`, {
        headers: { Cookie: owner, Range: 'bytes=99-100' }
      });
      assert.equal(res.status, 416);
    });

    await t.test('an unknown id is not found (404)', async () => {
      const res = await fetch(`${DOWNLOAD_URL}/00000000-0000-0000-0000-000000000000`, {
        headers: { Cookie: owner }
      });
      assert.equal(res.status, 404);
    });

    await t.test('a malformed id is rejected (400)', async () => {
      const res = await fetch(`${DOWNLOAD_URL}/not-a-uuid`, { headers: { Cookie: owner } });
      assert.equal(res.status, 400);
    });
  });
});
