import { NxpNode } from '../../NxpNode.js';

export default new NxpNode({
  name: 'authenticated_live_feed',
  description: 'Protected live stream endpoint. Requires valid authentication session cookie.',
  method: 'WS',
  messageSchema: {
    type: 'object',
    properties: {
      message: { type: 'string', minLength: 1 }
    },
    required: ['message'],
    additionalProperties: false
  },
  ws: {
    open: (ctx) => {
      const user = ctx.session.get('user');
      ctx.send({
        type: 'authorized',
        user: user?.username || 'Anonymous',
        role: user?.role || 'member',
        message: `Welcome to the secure live stream, ${user?.username}!`
      });
    },
    message: (ctx, data) => {
      const user = ctx.session.get('user');
      ctx.broadcast({
        type: 'chat',
        from: user?.username || 'Anonymous',
        role: user?.role || 'member',
        message: data.message,
        timestamp: new Date().toISOString()
      }, { includeSelf: true });
    }
  }
});
