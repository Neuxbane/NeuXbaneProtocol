import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'session_login',
  description: 'Sets stateful cookie session. Accepts username and optional role.',
  method: 'POST',
  schema: {
    request: {
      username: { type: 'string', required: true },
      role: { type: 'string', enum: ['member', 'admin'], default: 'member' }
    },
    response: {
      type: 'object',
      properties: {
        success: { type: 'boolean' },
        message: { type: 'string' }
      },
      required: ['success', 'message'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const username = ctx.params.username || 'Anonymous';
    const role = ctx.params.role === 'admin' ? 'admin' : 'member';

    ctx.session.set('user', { username, role, loginAt: new Date().toISOString() });

    return {
      success: true,
      message: `Session initialized for ${username} with role [${role}]`
    };
  }
});
