import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
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

test('Resumable chunked upload (/__nxp/upload)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('requires auth (401)', async () => {
      const res = await fetch(UPLOAD_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ originalName: 'x.bin', size: 4 })
      });
      assert.equal(res.status, 401);
    });

    await t.test('init returns chunk plan', async () => {
      const { res, data } = await initSession(cookie, {
        originalName: 'big.bin',
        size: 12,
        chunkSize: 4,
        mimeType: 'application/octet-stream'
      });
      assert.equal(res.status, 200);
      assert.equal(data.totalChunks, 3);
      assert.equal(data.chunkSize, 4);
      assert.deepEqual(data.received, []);
      assert.deepEqual(data.missing, [0, 1, 2]);
      assert.equal(data.complete, false);
    });

    await t.test('out-of-order chunks + status reports missing (resume)', async () => {
      const { data: session } = await initSession(cookie, {
        originalName: 'resume.bin',
        size: 12,
        chunkSize: 4
      });
      const id = session.uploadId;

      const put = await putChunk(cookie, id, 2, Buffer.from('CCCC'));
      assert.equal(put.res.status, 200);
      assert.deepEqual(put.data.received, [2]);
      assert.deepEqual(put.data.missing, [0, 1]);
      assert.equal(put.data.complete, false);

      const statusRes = await fetch(`${UPLOAD_URL}/${id}`, { headers: { Cookie: cookie } });
      const status = await statusRes.json();
      assert.deepEqual(status.missing, [0, 1]);
      assert.equal(status.bytesReceived, 4);

      // Re-put the same chunk is idempotent (no duplicate).
      const again = await putChunk(cookie, id, 2, Buffer.from('CCCC'));
      assert.deepEqual(again.data.received, [2]);

      await fetch(`${UPLOAD_URL}/${id}/abort`, { method: 'DELETE', headers: { Cookie: cookie } });
    });

    await t.test('complete assembles chunks in order with correct checksum', async () => {
      const payload = 'AAAABBBBCCCC';
      const { data: session } = await initSession(cookie, {
        originalName: 'assemble.bin',
        size: payload.length,
        chunkSize: 4,
        mimeType: 'application/octet-stream'
      });
      const id = session.uploadId;

      // Upload in a scrambled order.
      await putChunk(cookie, id, 1, Buffer.from('BBBB'));
      await putChunk(cookie, id, 2, Buffer.from('CCCC'));
      await putChunk(cookie, id, 0, Buffer.from('AAAA'));

      const completeRes = await fetch(`${UPLOAD_URL}/${id}/complete`, {
        method: 'POST',
        headers: { Cookie: cookie }
      });
      assert.equal(completeRes.status, 200);
      const { file } = await completeRes.json();
      assert.equal(file.size, payload.length);
      assert.equal(
        file.checksum,
        `sha256:${crypto.createHash('sha256').update(payload).digest('hex')}`
      );

      // The assembled file is downloadable and byte-identical.
      const dl = await fetch(`${BASE_URL}/auth/upload/download?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(dl.status, 200);
      assert.equal(await dl.text(), payload);

      // Session is gone after finalize.
      const gone = await fetch(`${UPLOAD_URL}/${id}`, { headers: { Cookie: cookie } });
      assert.equal(gone.status, 404);

      await fetch(`${BASE_URL}/auth/upload/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: file.id })
      });
    });

    await t.test('complete fails while chunks are missing (409)', async () => {
      const { data: session } = await initSession(cookie, {
        originalName: 'partial.bin',
        size: 8,
        chunkSize: 4
      });
      const id = session.uploadId;
      await putChunk(cookie, id, 0, Buffer.from('AAAA'));

      const res = await fetch(`${UPLOAD_URL}/${id}/complete`, {
        method: 'POST',
        headers: { Cookie: cookie }
      });
      assert.equal(res.status, 409);
      const data = await res.json();
      assert.match(data.error, /Missing chunk/);

      await fetch(`${UPLOAD_URL}/${id}/abort`, { method: 'DELETE', headers: { Cookie: cookie } });
    });

    await t.test('abort discards the session', async () => {
      const { data: session } = await initSession(cookie, {
        originalName: 'abort.bin',
        size: 4,
        chunkSize: 4
      });
      const id = session.uploadId;

      const abortRes = await fetch(`${UPLOAD_URL}/${id}/abort`, {
        method: 'DELETE',
        headers: { Cookie: cookie }
      });
      assert.equal(abortRes.status, 200);

      const statusRes = await fetch(`${UPLOAD_URL}/${id}`, { headers: { Cookie: cookie } });
      assert.equal(statusRes.status, 404);
    });

    await t.test('rejects an out-of-range chunk index (400)', async () => {
      const { data: session } = await initSession(cookie, {
        originalName: 'range.bin',
        size: 4,
        chunkSize: 4
      });
      const id = session.uploadId;

      const res = await putChunk(cookie, id, 5, Buffer.from('XXXX'));
      assert.equal(res.res.status, 400);

      await fetch(`${UPLOAD_URL}/${id}/abort`, { method: 'DELETE', headers: { Cookie: cookie } });
    });

    await t.test('rejects a chunk with the wrong byte length (400)', async () => {
      const { data: session } = await initSession(cookie, {
        originalName: 'len.bin',
        size: 8,
        chunkSize: 4
      });
      const id = session.uploadId;

      const res = await putChunk(cookie, id, 0, Buffer.from('AB'));
      assert.equal(res.res.status, 400);

      await fetch(`${UPLOAD_URL}/${id}/abort`, { method: 'DELETE', headers: { Cookie: cookie } });
    });

    await t.test('rejects a file larger than the cap (413)', async () => {
      const { res } = await initSession(cookie, {
        originalName: 'huge.bin',
        size: 999 * 1024 * 1024,
        chunkSize: 1024
      });
      assert.equal(res.status, 413);
    });

    await t.test('lists active sessions', async () => {
      const { data: session } = await initSession(cookie, {
        originalName: 'listed.bin',
        size: 4,
        chunkSize: 4
      });

      const res = await fetch(UPLOAD_URL, { headers: { Cookie: cookie } });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.ok(data.sessions.some((s) => s.uploadId === session.uploadId));

      await fetch(`${UPLOAD_URL}/${session.uploadId}/abort`, {
        method: 'DELETE',
        headers: { Cookie: cookie }
      });
    });
  });
});

test('Resumable download (HTTP Range)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');
    const payload = '0123456789';

    const form = new FormData();
    form.append('file', new Blob([payload], { type: 'text/plain' }), 'range.txt');
    const uploadRes = await fetch(`${BASE_URL}/auth/upload/file`, {
      method: 'POST',
      headers: { Cookie: cookie },
      body: form
    });
    const file = (await uploadRes.json()).files[0];

    await t.test('advertises Accept-Ranges', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=${file.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(res.headers.get('accept-ranges'), 'bytes');
    });

    await t.test('serves a byte range with 206', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=${file.id}`, {
        headers: { Cookie: cookie, Range: 'bytes=2-5' }
      });
      assert.equal(res.status, 206);
      assert.equal(res.headers.get('content-range'), `bytes 2-5/${payload.length}`);
      assert.equal(await res.text(), '2345');
    });

    await t.test('serves a suffix range', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=${file.id}`, {
        headers: { Cookie: cookie, Range: 'bytes=-3' }
      });
      assert.equal(res.status, 206);
      assert.equal(await res.text(), '789');
    });

    await t.test('rejects an unsatisfiable range with 416', async () => {
      const res = await fetch(`${BASE_URL}/auth/upload/download?id=${file.id}`, {
        headers: { Cookie: cookie, Range: 'bytes=999-1000' }
      });
      assert.equal(res.status, 416);
    });

    await fetch(`${BASE_URL}/auth/upload/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: file.id })
    });
  });
});
