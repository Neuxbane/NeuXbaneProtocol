import { NxpNode } from '../../NxpNode.js';

export default new NxpNode({
  name: 'user_profile',
  description: 'Displays current user profile details.',
  method: 'GET',
  schema: {
    response: {
      type: 'object',
      properties: {
        profile: {
          anyOf: [
            { type: 'null' },
            {
              type: 'object',
              properties: {
                username: { type: 'string' },
                role: { type: 'string' },
                loginAt: { type: 'string' }
              },
              required: ['username', 'role']
            }
          ]
        },
        status: { type: 'string' }
      },
      required: ['profile', 'status'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    profile: ctx.session.get('user'),
    status: 'Active'
  })
});
