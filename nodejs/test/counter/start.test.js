import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, withCounterLock } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /counter/start (define/counter/start.js)', async (t) => {
  await withCounterLock(async () => {
    // First ensure counter is reset and stopped
    await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
    await fetch(`${BASE_URL}/counter/reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ to: 0 })
    });

    await t.test('POST /counter/start starts counting in background', async () => {
      const res = await fetch(`${BASE_URL}/counter/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ intervalMs: 80 })
      });
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.success, true);
      assert.equal(data.running, true);

      // Wait 250ms to allow ~3 background ticks
      await new Promise((resolve) => setTimeout(resolve, 250));

      // Verify counter status updated in background
      const checkRes = await fetch(`${BASE_URL}/counter`);
      const checkData = await checkRes.json();
      assert.equal(checkData.running, true);
      assert.ok(checkData.count >= 2, `Expected count >= 2, got ${checkData.count}`);

      // Clean up: stop counter
      await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
    });

    await t.test('GET /counter/start?nxp returns manifest', async () => {
      const res = await fetch(`${BASE_URL}/counter/start?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'counter_start');
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
