import test from 'node:test';
import assert from 'node:assert/strict';
import { NxpNode, NXP_MODE, isDev } from '../NxpNode.js';
import { BASE_URL, ensureServerRunning } from './helper.js';

await ensureServerRunning();

test('Response schema contract (dev mode)', async (t) => {
  await t.test('server reports dev mode in the manifest', async () => {
    const res = await fetch(`${BASE_URL}/?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.equal(data.mode, 'dev');
  });

  await t.test('manifest exposes schema.request and schema.response', async () => {
    const res = await fetch(`${BASE_URL}/login?nxp`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.ok(data.schema, 'expected a schema on /login');
    assert.ok(data.schema.request, 'expected schema.request');
    assert.ok(data.schema.response, 'expected schema.response');
    assert.equal(data.schema.response.type, 'object');
    assert.ok(data.schema.response.properties.success);
  });

  await t.test('valid responses pass through unchanged', async () => {
    const res = await fetch(`${BASE_URL}/`);
    assert.equal(res.status, 200);
    const data = await res.json();
    assert.match(data.message, /Neuxbane Protocol Online/);
  });
});

test('NxpNode.validateResponse (unit)', async (t) => {
  await t.test('returns data unchanged when no responseSchema is declared', () => {
    const node = new NxpNode({ name: 'no_schema', description: 'x' });
    const payload = { anything: true };
    assert.equal(node.validateResponse(payload), payload);
  });

  await t.test('accepts a conforming response', () => {
    const node = new NxpNode({
      name: 'ok',
      description: 'x',
      schema: {
        response: {
          type: 'object',
          properties: { success: { type: 'boolean' } },
          required: ['success'],
          additionalProperties: false
        }
      }
    });
    const payload = { success: true };
    assert.equal(node.validateResponse(payload), payload);
  });

  await t.test('throws a 500 with details on a violating response', () => {
    const node = new NxpNode({
      name: 'bad',
      description: 'x',
      schema: {
        response: {
          type: 'object',
          properties: { success: { type: 'boolean' } },
          required: ['success'],
          additionalProperties: false
        }
      }
    });

    assert.throws(
      () => node.validateResponse({ success: 'yes' }),
      (error) => {
        assert.match(error.message, /^500:/);
        assert.match(error.message, /Response schema violation/);
        assert.match(error.message, /success/);
        return true;
      }
    );
  });

  await t.test('supports shorthand response schemas', () => {
    const node = new NxpNode({
      name: 'shorthand',
      description: 'x',
      schema: {
        response: {
          message: { type: 'string', required: true }
        }
      }
    });
    assert.equal(node.validateResponse({ message: 'hi' }).message, 'hi');
    assert.throws(() => node.validateResponse({}), /^Error: 500:/);
  });

  await t.test('a bare schema is treated as the request contract', () => {
    const node = new NxpNode({
      name: 'bare',
      description: 'x',
      schema: {
        username: { type: 'string', required: true }
      }
    });
    assert.equal(node.responseSchema, null);
    assert.equal(node.validateResponse({ anything: true }).anything, true);
  });

  await t.test('mode is dev by default', () => {
    assert.equal(NXP_MODE, 'dev');
    assert.equal(isDev, true);
  });
});
