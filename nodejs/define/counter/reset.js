import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_reset',
  description: 'Resets the background counter value.',
  method: 'POST',
  schema: {
    type: 'object',
    properties: {
      to: { type: 'integer', default: 0 }
    },
    additionalProperties: false
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
