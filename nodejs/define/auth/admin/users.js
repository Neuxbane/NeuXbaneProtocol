import { NxpNode } from '../../../NxpNode.js';

export default new NxpNode({
  name: 'list_admin_users',
  description: 'Returns internal directory. Requires /auth and /auth/admin guards.',
  method: 'GET',
  schema: {
    response: {
      type: 'object',
      properties: {
        adminViewer: { type: 'string' },
        users: {
          type: 'array',
          items: {
            type: 'object',
            properties: {
              id: { type: 'integer' },
              username: { type: 'string' },
              role: { type: 'string' }
            },
            required: ['id', 'username', 'role'],
            additionalProperties: false
          }
        }
      },
      required: ['adminViewer', 'users'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    adminViewer: ctx.session.get('user').username,
    users: [
      { id: 101, username: 'neuxbane', role: 'admin' },
      { id: 102, username: 'agent_smith', role: 'member' }
    ]
  })
});
