import test from 'node:test';
import assert from 'node:assert/strict';
import { BASE_URL, ensureServerRunning, withCounterLock } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /counter/stop (define/counter/stop.js)', async (t) => {
  await withCounterLock(async () => {
    // Stop, reset, and start fresh
    await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
    await fetch(`${BASE_URL}/counter/reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ to: 0 })
    });
    await fetch(`${BASE_URL}/counter/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ intervalMs: 80 })
    });

    // Wait a brief moment for it to tick
    await new Promise((resolve) => setTimeout(resolve, 200));

    // Stop counter
    const res = await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.success, true);
    assert.equal(data.running, false);

    const snapshot = data.count;
    assert.ok(snapshot >= 1, `Expected snapshot >= 1, got ${snapshot}`);

    // Wait another 150ms and verify count didn't increase
    await new Promise((resolve) => setTimeout(resolve, 150));
    const checkRes = await fetch(`${BASE_URL}/counter`);
    const checkData = await checkRes.json();
    assert.equal(checkData.count, snapshot);
    assert.equal(checkData.running, false);

    await t.test('GET /counter/stop?nxp returns manifest', async () => {
      const res = await fetch(`${BASE_URL}/counter/stop?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'counter_stop');
      assert.equal(data.method, 'POST');
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
