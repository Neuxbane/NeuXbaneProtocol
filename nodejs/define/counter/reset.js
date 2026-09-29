import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_reset',
  description: 'Resets the background counter value.',
  method: 'POST',
  schema: {
    request: {
      type: 'object',
      properties: {
        to: { type: 'integer', default: 0 }
      },
      additionalProperties: false
    },
    response: {
      type: 'object',
      properties: {
        success: { type: 'boolean' },
        message: { type: 'string' },
        count: { type: 'integer' },
        running: { type: 'boolean' },
        intervalMs: { type: 'integer' }
      },
      required: ['success', 'message', 'count', 'running', 'intervalMs'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const to = ctx.params.to ?? 0;
    const state = counterService.reset(to);
    return {
      success: true,
      message: `Counter reset to ${to}`,
      ...state
    };
  }
});
