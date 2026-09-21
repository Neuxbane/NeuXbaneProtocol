import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'root_portal',
  description: 'Global entrypoint and system directory.',
  handler: async (ctx) => ({
    message: 'Neuxbane Protocol Online',
    authenticatedAs: ctx.session.get('user') || null,
    routes: Array.from(ctx.targetNode.children.keys())
  })
});
