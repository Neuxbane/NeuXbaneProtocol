import test from 'node:test';
import assert from 'node:assert/strict';
import { WebSocket } from 'ws';
import { BASE_URL, WS_URL, ensureServerRunning, getAuthCookie } from '../helper.js';

await ensureServerRunning();

test('Endpoint: /auth/live (define/auth/live.js)', async (t) => {
  await t.test('WS /auth/live rejects unauthenticated connection with 401', async () => {
    await new Promise((resolve) => {
      const ws = new WebSocket(`${WS_URL}/auth/live`);
      ws.on('unexpected-response', (req, res) => {
        assert.equal(res.statusCode, 401);
        resolve();
      });
      ws.on('open', () => {
        assert.fail('Should not connect without cookie');
      });
    });
  });

  await t.test('WS /auth/live connects with valid session cookie', async () => {
    const cookie = await getAuthCookie('alice', 'admin');
    const ws = new WebSocket(`${WS_URL}/auth/live`, {
      headers: { Cookie: cookie }
    });

    const messages = [];
    await new Promise((resolve, reject) => {
      ws.on('open', () => {
        ws.send(JSON.stringify({ action: 'message', message: 'Hello secure channel' }));
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

    assert.equal(messages[0].type, 'authorized');
    assert.equal(messages[0].user, 'alice');
    assert.equal(messages[0].role, 'admin');

    assert.equal(messages[1].type, 'chat');
    assert.equal(messages[1].from, 'alice');
    assert.equal(messages[1].message, 'Hello secure channel');
  });

  await t.test('GET /auth/live?nxp describes WebSocket endpoint', async () => {
    const res = await fetch(`${BASE_URL}/auth/live?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.name, 'authenticated_live_feed');
    assert.equal(data.isWebSocket, true);
    assert.ok(Array.isArray(data.inheritedGuards));
  });
});
