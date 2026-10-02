import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_get',
  description: 'Returns the current background counter state.',
  method: 'GET',
  schema: {
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
  handler: async () => {
    const state = counterService.getState();
    return {
      success: true,
      message: 'Counter state retrieved',
      ...state
    };
  }
});
