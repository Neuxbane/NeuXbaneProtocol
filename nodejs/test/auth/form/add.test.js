import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, getAuthCookie, withLock } from '../../helper.js';
import { FORM_LIMIT } from '../../../services/formService.js';

await ensureServerRunning();

async function add(cookie, items) {
  return fetch(`${BASE_URL}/auth/form/add`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ items })
  });
}

test('Endpoint: /auth/form/add (define/auth/form/add.js)', async (t) => {
  await withLock('form', async () => {
    const cookie = await getAuthCookie('alice', 'member');

    const clear = () => fetch(`${BASE_URL}/auth/form/clear`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ target: 'bounded' })
    });

    await t.test('POST /auth/form/add requires auth guard (401)', async () => {
      const res = await fetch(`${BASE_URL}/auth/form/add`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items: ['x'] })
      });
      assert.equal(res.status, 401);
    });

    await t.test('POST /auth/form/add rejects a missing or empty items array (400)', async () => {
      await clear();

      const missing = await add(cookie, undefined);
      assert.equal(missing.status, 400);

      const empty = await add(cookie, []);
      assert.equal(empty.status, 400);
    });

    await t.test(`POST /auth/form/add rejects more than maxItems (${FORM_LIMIT}) (400)`, async () => {
      await clear();
      const tooMany = Array.from({ length: FORM_LIMIT + 1 }, (_, i) => `entry-${i}`);
      const res = await add(cookie, tooMany);
      assert.equal(res.status, 400);
      const data = await res.json();
      assert.match(data.error, /maxItems|more than/i);
    });

    await t.test('POST /auth/form/add accepts an array of objects and strings', async () => {
      await clear();
      const res = await add(cookie, [{ name: 'Ada' }, 'Grace', 42]);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.added.length, 3);
      assert.deepEqual(data.added[0].value, { name: 'Ada' });
      assert.equal(data.entries.length, 3);
      assert.equal(data.limit, FORM_LIMIT);
      assert.equal(data.remaining, FORM_LIMIT - 3);
    });

    await t.test('POST /auth/form/add enforces the running total, not just the batch size', async () => {
      await clear();
      // Two batches of 3 each: both pass maxItems, but the second overflows.
      assert.equal((await add(cookie, ['a', 'b', 'c'])).status, 200);
      const res = await add(cookie, ['d', 'e', 'f']);
      assert.equal(res.status, 400);
      const data = await res.json();
      assert.match(data.error, /Limit exceeded/);

      // Nothing from the rejected batch should have been stored.
      const state = await fetch(`${BASE_URL}/auth/form`, { headers: { Cookie: cookie } });
      assert.equal((await state.json()).bounded.count, 3);
    });

    await clear();

    await t.test('POST /auth/form/add?nxp exposes the array limit in the manifest', async () => {
      const res = await fetch(`${BASE_URL}/auth/form/add?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'form_add_bounded');
      const items = data.schema.request.properties.items;
      assert.equal(items.type, 'array');
      assert.equal(items.maxItems, FORM_LIMIT);
      assert.ok(items.items.oneOf);
    });
  });
});
