import { NxpNode, arrayOf } from '../../../NxpNode.js';
import { unboundedFormService } from '../../../services/formService.js';

/**
 * Unlimited collection: the input array has no `maxItems`, so any number of
 * entries can be appended in a single request.
 */
export default new NxpNode({
  name: 'form_add_unbounded',
  description: 'Appends an array of entries to the unbounded collection (no limit).',
  method: 'POST',
  schema: {
    request: {
      type: 'object',
      properties: {
        items: arrayOf(
          { oneOf: [{ type: 'object' }, { type: 'string' }, { type: 'number' }] },
          { minItems: 1 }
        )
      },
      required: ['items'],
      additionalProperties: false
    },
    response: {
      type: 'object',
      properties: {
        success: { type: 'boolean' },
        message: { type: 'string' },
        added: {
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
        },
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
      required: ['success', 'message', 'added', 'limit', 'unlimited', 'count', 'remaining', 'entries'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const added = unboundedFormService.add(ctx.params.items);
    return {
      success: true,
      message: `Added ${added.length} entr${added.length === 1 ? 'y' : 'ies'}.`,
      added,
      ...unboundedFormService.getState()
    };
  }
});
