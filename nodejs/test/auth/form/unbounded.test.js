import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';

await ensureServerRunning();

async function add(cookie, items) {
  return fetch(`${BASE_URL}/auth/form/unbounded`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ items })
  });
}

test('Endpoint: /auth/form/unbounded (define/auth/form/unbounded.js)', async (t) => {
  await withLock('form', async () => {
    const cookie = await getAuthCookie('alice', 'member');

    const clear = () => fetch(`${BASE_URL}/auth/form/clear`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ target: 'unbounded' })
    });

    await t.test('POST /auth/form/unbounded requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/form/unbounded`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items: ['x'] })
      });
      assert.equal(res.status, 401);
    });

    await t.test('POST /auth/form/unbounded accepts far more items than the bounded limit', async () => {
      await clear();
      const many = Array.from({ length: 25 }, (_, i) => ({ index: i }));
      const res = await add(cookie, many);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.added.length, 25);
      assert.equal(data.count, 25);
      assert.equal(data.unlimited, true);
      assert.equal(data.limit, null);
      assert.equal(data.remaining, null);

      await clear();
    });

    await t.test('POST /auth/form/unbounded still rejects an empty array (400)', async () => {
      await clear();
      const res = await add(cookie, []);
      assert.equal(res.status, 400);
    });

    await t.test('POST /auth/form/unbounded?nxp has no maxItems', async () => {
      const res = await fetch(`${BASE_URL}/auth/form/unbounded?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'form_add_unbounded');
      const items = data.schema.properties.items;
      assert.equal(items.type, 'array');
      assert.equal(items.maxItems, undefined);
    });
  });
});
