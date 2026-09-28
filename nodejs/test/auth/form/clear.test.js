import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/form/clear (define/auth/form/clear.js)', async (t) => {
  await withLock('form', async () => {
    const cookie = await getAuthCookie('alice', 'member');

    const clear = (target) => fetch(`${BASE_URL}/auth/form/clear`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify(target ? { target } : {})
    });

    const add = (path, items) => fetch(`${BASE_URL}/auth/form/${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ items })
    });

    await t.test('POST /auth/form/clear requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/form/clear`, { method: 'POST' });
      assert.equal(res.status, 401);
    });

    await t.test('POST /auth/form/clear empties the bounded collection (default target)', async () => {
      await add('add', ['one', 'two']);
      const res = await clear();
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.target, 'bounded');
      assert.equal(data.count, 0);
      assert.deepEqual(data.entries, []);
    });

    await t.test('POST /auth/form/clear empties the unbounded collection', async () => {
      await add('unbounded', ['a', 'b', 'c']);
      const res = await clear('unbounded');
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.target, 'unbounded');
      assert.equal(data.count, 0);
      assert.equal(data.unlimited, true);
    });

    await t.test('POST /auth/form/clear rejects an unknown target (400)', async () => {
      const res = await clear('bogus');
      assert.equal(res.status, 400);
    });

    await t.test('POST /auth/form/clear?nxp returns endpoint manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/form/clear?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'form_clear');
      assert.equal(data.method, 'POST');
    });
  });
});
