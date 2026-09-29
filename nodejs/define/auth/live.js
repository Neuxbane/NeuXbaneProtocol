import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'authenticated_live_feed',
  description: 'Protected live stream endpoint. Requires valid authentication session cookie.',
  method: 'WS',
  messageSchema: {
    type: 'object',
    properties: {
      action: { type: 'string', enum: ['message', 'get'] },
      message: { type: 'string', minLength: 1 }
    },
    required: ['action'],
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

      if (data.action === 'get') {
        ctx.send({
          type: 'count',
          ...counterService.getState()
        });
        return;
      }

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
