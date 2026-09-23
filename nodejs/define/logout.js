import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'session_logout',
  description: 'Destroys the current stateful session.',
  method: 'POST',
  handler: async (ctx) => {
    ctx.session.clear();
    return { success: true, message: 'Logged out successfully.' };
  }
});
