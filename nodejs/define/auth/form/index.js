import { NxpNode } from '../../../NxpNode.js';
import { formService, unboundedFormService } from '../../../services/formService.js';

export default new NxpNode({
  name: 'form_scope_root',
  description:
    'Authenticated multi-item form scope. Append arrays of entries to a bounded or unbounded collection.',
  schema: {
    response: {
      type: 'object',
      properties: {
        message: { type: 'string' },
        user: {
          type: 'object',
          properties: {
            username: { type: 'string' },
            role: { type: 'string' },
            loginAt: { type: 'string' }
          },
          required: ['username', 'role']
        },
        bounded: { $ref: '#/$defs/collection' },
        unbounded: { $ref: '#/$defs/collection' },
        children: { type: 'array', items: { type: 'string' } }
      },
      required: ['message', 'user', 'bounded', 'unbounded', 'children'],
      additionalProperties: false,
      $defs: {
        collection: {
          type: 'object',
          properties: {
            limit: { anyOf: [{ type: 'integer' }, { type: 'null' }] },
            unlimited: { type: 'boolean' },
            count: { type: 'integer' },
            remaining: { anyOf: [{ type: 'integer' }, { type: 'null' }] },
            entries: {
              type: 'array',
              items: {
                type: 'object',
                properties: {
                  id: { type: 'string' },
                  value: {},
                  createdAt: { type: 'string' }
                },
                required: ['id', 'createdAt']
              }
            }
          },
          required: ['limit', 'unlimited', 'count', 'remaining', 'entries']
        }
      }
    }
  },
  handler: async (ctx) => ({
    message: 'Multi-item form hub.',
    user: ctx.session.get('user'),
    bounded: formService.getState(),
    unbounded: unboundedFormService.getState(),
    children: Array.from(ctx.targetNode.children.keys())
  })
});
