import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

async function upload(cookie, name, contents) {
  const form = new FormData();
  form.append('file', new Blob([contents], { type: 'text/plain' }), name);
  const res = await fetch(`${BASE_URL}/auth/upload/file`, {
    method: 'POST',
    headers: { Cookie: cookie },
    body: form
  });
  assert.equal(res.status, 200);
  return (await res.json()).files[0];
}

async function remove(cookie, id) {
  await fetch(`${BASE_URL}/auth/upload/delete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ id })
  });
}

test('Content-addressed dedup (/auth/upload)', async (t) => {
  await withLock('upload', async () => {
    const cookie = await getAuthCookie('alice', 'admin');

    await t.test('identical content is stored once and shared by reference', async () => {
      const payload = `dedup-${crypto.randomUUID()}`;
      const first = await upload(cookie, 'one.txt', payload);
      const second = await upload(cookie, 'two.txt', payload);

      // Same bytes -> same checksum and same physical blob.
      assert.equal(first.checksum, second.checksum);
      assert.equal(first.storedName, second.storedName);
      assert.equal(second.deduplicated, true);

      // Deleting one record keeps the shared blob alive for the other.
      await remove(cookie, first.id);
      const dl = await fetch(`${BASE_URL}/auth/upload/download?id=${second.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(dl.status, 200);
      assert.equal(await dl.text(), payload);

      // Deleting the last reference releases the blob.
      await remove(cookie, second.id);
      const after = await fetch(`${BASE_URL}/auth/upload/download?id=${second.id}`, {
        headers: { Cookie: cookie }
      });
      assert.equal(after.status, 404);
    });

    await t.test('different content gets a distinct blob', async () => {
      const a = await upload(cookie, 'a.txt', `a-${crypto.randomUUID()}`);
      const b = await upload(cookie, 'b.txt', `b-${crypto.randomUUID()}`);
      assert.notEqual(a.storedName, b.storedName);
      await remove(cookie, a.id);
      await remove(cookie, b.id);
    });
  });
});
