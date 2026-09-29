import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../helper.js';

await ensureServerRunning();

const UPLOAD_URL = `${BASE_URL}/__nxp/upload`;
const LINK_URL = `${BASE_URL}/__nxp/link`;

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

test('Stateless signed file links (/__nxp/link)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('a signed link streams the file without a cookie', async () => {
      const payload = 'signed-link-payload';
      const file = await storeFile(cookie, 'link.bin', payload);

      // Build a link via the response-file contract on a JSON route.
      const linkRes = await fetch(`${BASE_URL}/__nxp/link-test?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(linkRes.status, 200);
      const { link } = await linkRes.json();
      assert.match(link.url, /^\/__nxp\/link\//);

      // Redeem the link with NO cookie — the token is the capability.
      const dl = await fetch(`${BASE_URL}${link.url}`);
      assert.equal(dl.status, 200);
      assert.equal(await dl.text(), payload);
    });

    await t.test('a tampered token is rejected with 403', async () => {
      const file = await storeFile(cookie, 'tamper.bin', 'tamper-me');
      const linkRes = await fetch(`${BASE_URL}/__nxp/link-test?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      const { link } = await linkRes.json();

      // Flip the last character of the signature.
      const token = link.url.split('/').pop();
      const tampered = token.slice(0, -1) + (token.endsWith('A') ? 'B' : 'A');

      const res = await fetch(`${LINK_URL}/${tampered}`);
      assert.equal(res.status, 403);
    });

    await t.test('a malformed token is rejected with 403', async () => {
      const res = await fetch(`${LINK_URL}/not-a-real-token`);
      assert.equal(res.status, 403);
    });

    await t.test('the emitted link carries an ISO expiry in the future', async () => {
      const file = await storeFile(cookie, 'expiry.bin', 'expiry-check');
      const linkRes = await fetch(`${BASE_URL}/__nxp/link-test?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      const { link } = await linkRes.json();
      assert.equal(link.id, file.id);
      const exp = Date.parse(link.expiresAt);
      assert.ok(Number.isFinite(exp), 'expiresAt should be a parseable date');
      assert.ok(exp > Date.now(), 'expiresAt should be in the future');
    });
  });
});
