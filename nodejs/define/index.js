import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'root_portal',
  description: 'Global entrypoint and system directory.',
  schema: {
    response: {
      type: 'object',
      properties: {
        message: { type: 'string' },
        authenticatedAs: {
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
        routes: { type: 'array', items: { type: 'string' } }
      },
      required: ['message', 'routes'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    message: 'Neuxbane Protocol Online',
    authenticatedAs: ctx.session.get('user') || null,
    routes: Array.from(ctx.targetNode.children.keys())
  })
});
