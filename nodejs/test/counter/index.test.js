import test from 'node:test';
import assert from 'node:assert/strict';
import { WebSocket } from 'ws';
import { BASE_URL, WS_URL, ensureServerRunning, withCounterLock } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /counter (define/counter/index.js)', async (t) => {
  await withCounterLock(async () => {
    // Ensure counter is stopped before tests
    await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
    await fetch(`${BASE_URL}/counter/reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ to: 0 })
    });

    await t.test('GET /counter returns runtime counter status', async () => {
      const res = await fetch(`${BASE_URL}/counter`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.match(data.message, /Runtime counter status/);
      assert.ok(typeof data.count === 'number');
      assert.ok(typeof data.running === 'boolean');
    });

    await t.test('GET /counter?nxp returns self-describing manifest', async () => {
      const res = await fetch(`${BASE_URL}/counter?nxp`);
      assert.equal(res.status, 200);
      const data = await res.json();
      assert.equal(data.name, 'counter_root');
      assert.equal(data.isWebSocket, true);
      assert.ok(data.messageSchema);
      assert.ok(data.children.start);
      assert.ok(data.children.stop);
      assert.ok(data.children.reset);
    });

    await t.test('WS /counter connects, receives initial state, streams ticks, and handles WS commands', async () => {
      const ws = new WebSocket(`${WS_URL}/counter`);
      const received = [];

      await new Promise((resolve, reject) => {
        ws.on('open', () => {
          ws.send(JSON.stringify({ action: 'reset', to: 50 }));
        });

        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString());
          received.push(msg);

          if (msg.type === 'reset') {
            ws.send(JSON.stringify({ action: 'start', intervalMs: 80 }));
          }

          const ticks = received.filter((m) => m.type === 'tick');
          if (ticks.length >= 2) {
            ws.send(JSON.stringify({ action: 'stop' }));
          }

          if (msg.type === 'stopped') {
            ws.close();
          }
        });

        ws.on('close', resolve);
        ws.on('error', reject);
      });

      const resetMsg = received.find((m) => m.type === 'reset');
      assert.ok(resetMsg);
      assert.equal(resetMsg.count, 50);

      const ticks = received.filter((m) => m.type === 'tick');
      assert.ok(ticks.length >= 2, `Expected >= 2 ticks, got ${ticks.length}`);
      assert.ok(ticks[0].count >= 51);

      const stoppedMsg = received.find((m) => m.type === 'stopped');
      assert.ok(stoppedMsg);
      assert.equal(stoppedMsg.running, false);

      // Clean up
      await fetch(`${BASE_URL}/counter/stop`, { method: 'POST' });
      await fetch(`${BASE_URL}/counter/reset`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ to: 0 })
      });
    });
  });
});
