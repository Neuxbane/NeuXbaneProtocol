import { NxpNode } from '../../NxpNode.js';

export default new NxpNode({
  name: 'auth_scope_root',
  description: 'Gatekeeper for /auth. Requires an active authenticated session.',
  guard: async (ctx) => {
    const user = ctx.session.get('user');
    if (!user) {
      throw new Error('401: Unauthorized - You must log in via /login first.');
    }
  },
  schema: {
    response: {
      type: 'object',
      properties: {
        message: { type: 'string' },
        user: {
          type: 'object',
          properties: {
            username: { type: 'string' },
            role: { type: 'string' },
            loginAt: { type: 'string' }
          },
          required: ['username', 'role']
        }
      },
      required: ['message', 'user'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    message: 'Welcome to the authenticated hub.',
    user: ctx.session.get('user')
  })
});
