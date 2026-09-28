import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, withCounterLock } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /counter/reset (define/counter/reset.js)', async (t) => {
  await withCounterLock(async () => {
    // Ensure counter is stopped before reset tests
    await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });

    await t.test('POST /counter/reset resets counter to specified value', async () => {
      const res = await fetch(`${BASE_URL}/counter/reset`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ to: 100 })
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.count, 100);

      const checkRes = await fetch(`${BASE_URL}/counter`);
      const checkData = await checkRes.json();
      assert.equal(checkData.count, 100);
    });

    await t.test('POST /counter/reset defaults to 0 when omitted', async () => {
      const res = await fetch(`${BASE_URL}/counter/reset`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.count, 0);
    });

    await t.test('GET /counter/reset?nxp returns manifest', async () => {
      const res = await fetch(`${BASE_URL}/counter/reset?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'counter_reset');
      assert.equal(data.method, 'POST');
      assert.ok(data.schema);
    });

    // Clean up
    await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
    await fetch(`${BASE_URL}/counter/reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ to: 0 })
    });
  });
});
