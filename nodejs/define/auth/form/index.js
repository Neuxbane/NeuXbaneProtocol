import { NxpNode } from '../../../NxpNode.js';
import { formService, unboundedFormService } from '../../../services/formService.js';

export default new NxpNode({
  name: 'form_scope_root',
  description:
    'Authenticated multi-item form scope. Append arrays of entries to a bounded or unbounded collection.',
  handler: async (ctx) => ({
    message: 'Multi-item form hub.',
    user: ctx.session.get('user'),
    bounded: formService.getState(),
    unbounded: unboundedFormService.getState(),
    children: Array.from(ctx.targetNode.children.keys())
  })
});
