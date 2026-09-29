import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_start',
  description: 'Starts the background counting script.',
  method: 'POST',
  schema: {
    request: {
      type: 'object',
      properties: {
        intervalMs: { type: 'integer', minimum: 50, default: 1000 }
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
    const intervalMs = ctx.params.intervalMs || 1000;
    const state = counterService.start(intervalMs);
    return {
      success: true,
      message: `Counter started with interval ${state.intervalMs}ms`,
      ...state
    };
  }
});
