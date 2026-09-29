import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_stop',
  description: 'Stops the background counting script.',
  method: 'POST',
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
    const state = counterService.stop();
    return {
      success: true,
      message: 'Counter stopped',
      ...state
    };
  }
});
