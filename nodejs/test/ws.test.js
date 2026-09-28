import test from 'node:test';
import assert from 'node:assert/strict';
import { WebSocket } from 'ws';
import { BASE_URL, WS_URL, ensureServerRunning } from './helper.js';

await ensureServerRunning();

test('Endpoint: /ws (define/ws.js)', async (t) => {
  await t.test('GET /ws?nxp returns WebSocket manifest', async () => {
    const res = await fetch(`${BASE_URL}/ws?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'public_stream');
    assert.equal(data.method, 'WS');
    assert.equal(data.isWebSocket, true);
    assert.ok(data.messageSchema);
  });

  await t.test('GET /ws without upgrade header returns 426', async () => {
    const res = await fetch(`${BASE_URL}/ws`);
    assert.equal(res.status, 426);
    const data = await res.json();
    assert.match(data.error, /WebSocket endpoint/i);
  });

  await t.test('WS /ws connects, receives welcome, and executes echo', async () => {
    const ws = new WebSocket(`${WS_URL}/ws?clientName=Tester`);
    const messages = [];

    await new Promise((resolve, reject) => {
      ws.on('open', () => {
        ws.send(JSON.stringify({ action: 'echo', text: 'Hello NXP' }));
      });
      ws.on('message', (data) => {
        messages.push(JSON.parse(data.toString()));
        if (messages.length === 2) {
          ws.close();
        }
      });
      ws.on('close', resolve);
      ws.on('error', reject);
    });

    assert.equal(messages[0].type, 'connected');
    assert.match(messages[0].message, /Tester/);
    assert.equal(messages[1].type, 'echo');
    assert.equal(messages[1].text, 'Hello NXP');
  });

  await t.test('WS /ws validates message schema', async () => {
    const ws = new WebSocket(`${WS_URL}/ws`);
    const messages = [];

    await new Promise((resolve, reject) => {
      ws.on('open', () => {
        ws.send(JSON.stringify({ invalid: 'payload' }));
      });
      ws.on('message', (data) => {
        const msg = JSON.parse(data.toString());
        messages.push(msg);
        if (msg.error) {
          ws.close();
        }
      });
      ws.on('close', resolve);
      ws.on('error', reject);
    });

    const err = messages.find((m) => m.error);
    assert.ok(err);
    assert.match(err.error, /400/);
  });

  await t.test('WS /ws broadcast sends to other clients', async () => {
    const c1 = new WebSocket(`${WS_URL}/ws?clientName=Sender`);
    await new Promise((res) => c1.on('open', res));

    const c2 = new WebSocket(`${WS_URL}/ws?clientName=Receiver`);
    await new Promise((res) => c2.on('open', res));

    const broadcastPromise = new Promise((resolve) => {
      c2.on('message', (data) => {
        const msg = JSON.parse(data.toString());
        if (msg.type === 'broadcast') resolve(msg);
      });
    });

    c1.send(JSON.stringify({ action: 'broadcast', text: 'Live announcement' }));
    const broadcastMsg = await broadcastPromise;

    assert.equal(broadcastMsg.type, 'broadcast');
    assert.equal(broadcastMsg.sender, 'Sender');
    assert.equal(broadcastMsg.text, 'Live announcement');

    c1.close();
    c2.close();
    await Promise.all([
      new Promise((res) => c1.on('close', res)),
      new Promise((res) => c2.on('close', res))
    ]);
  });
});
