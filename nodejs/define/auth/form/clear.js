import { NxpNode } from '../../../NxpNode.js';
import { formService, unboundedFormService } from '../../../services/formService.js';

export default new NxpNode({
  name: 'form_clear',
  description: 'Clears stored entries. Pass `{ "target": "bounded" | "unbounded" }`, defaults to bounded.',
  method: 'POST',
  schema: {
    target: { type: 'string', enum: ['bounded', 'unbounded'], default: 'bounded' }
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
