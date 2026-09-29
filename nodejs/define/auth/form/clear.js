import { NxpNode } from '../../../NxpNode.js';
import { formService, unboundedFormService } from '../../../services/formService.js';

export default new NxpNode({
  name: 'form_clear',
  description: 'Clears stored entries. Pass `{ "target": "bounded" | "unbounded" }`, defaults to bounded.',
  method: 'POST',
  schema: {
    request: {
      target: { type: 'string', enum: ['bounded', 'unbounded'], default: 'bounded' }
    },
    response: {
      type: 'object',
      properties: {
        success: { type: 'boolean' },
        message: { type: 'string' },
        target: { type: 'string' },
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
      required: ['success', 'message', 'target', 'limit', 'unlimited', 'count', 'remaining', 'entries'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const target = ctx.params.target === 'unbounded' ? 'unbounded' : 'bounded';
    const service = target === 'unbounded' ? unboundedFormService : formService;
    const state = service.clear();

    return {
      success: true,
      message: `Cleared the ${target} collection.`,
      target,
      ...state
    };
  }
});
