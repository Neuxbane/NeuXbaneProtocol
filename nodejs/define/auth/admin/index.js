import { NxpNode } from '../../../NxpNode.js';

export default new NxpNode({
  name: 'admin_scope_root',
  description: 'Restricted administrative barrier. Requires user.role === "admin".',
  guard: async (ctx) => {
    const user = ctx.session.get('user');
    if (user.role !== 'admin') {
      throw new Error('403: Forbidden - Administrator privilege required.');
    }
  },
  schema: {
    response: {
      type: 'object',
      properties: {
        message: { type: 'string' },
        admin: { type: 'string' }
      },
      required: ['message', 'admin'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    message: 'Admin control center accessed.',
    admin: ctx.session.get('user').username
  })
});
