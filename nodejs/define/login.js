import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'session_login',
  description: 'Sets stateful cookie session. Accepts username and optional role.',
  method: 'POST',
  schema: {
    username: { type: 'string', required: true },
    role: { type: 'string', enum: ['member', 'admin'], default: 'member' }
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
