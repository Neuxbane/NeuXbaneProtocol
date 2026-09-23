import { NxpNode } from '../../NxpNode.js';

export default new NxpNode({
  name: 'user_profile',
  description: 'Displays current user profile details.',
  method: 'GET',
  handler: async (ctx) => ({
    profile: ctx.session.get('user'),
    status: 'Active'
  })
});
