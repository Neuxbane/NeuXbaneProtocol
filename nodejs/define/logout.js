import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'session_logout',
  description: 'Destroys the current stateful session.',
  method: 'POST',
  schema: {
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
    ctx.session.clear();
    return { success: true, message: 'Logged out successfully.' };
  }
});
