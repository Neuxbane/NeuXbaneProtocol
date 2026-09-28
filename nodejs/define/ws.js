import { NxpNode } from '../NxpNode.js';

export default new NxpNode({
  name: 'public_stream',
  description: 'Public real-time WebSocket channel for broadcasting and echo messaging.',
  method: 'WS',
  schema: {
    type: 'object',
    properties: {
      clientName: { type: 'string', default: 'Anonymous' }
    }
  },
  messageSchema: {
    type: 'object',
    properties: {
      action: { type: 'string', enum: ['echo', 'broadcast'] },
      text: { type: 'string', minLength: 1 }
    },
    required: ['action', 'text'],
    additionalProperties: false
  },
  ws: {
    open: (ctx) => {
      const clientName = ctx.params.clientName || 'Anonymous';
      ctx.send({
        type: 'connected',
        message: `Welcome ${clientName} to Neuxbane Protocol WebSocket stream!`,
        totalClients: ctx.clients.size
      });
      ctx.broadcast({
        type: 'join',
        message: `${clientName} has joined the stream.`,
        totalClients: ctx.clients.size
      });
    },
    message: (ctx, data) => {
      const clientName = ctx.params.clientName || 'Anonymous';
      if (data.action === 'echo') {
        ctx.send({ type: 'echo', text: data.text });
      } else if (data.action === 'broadcast') {
        ctx.broadcast({
          type: 'broadcast',
          sender: clientName,
          text: data.text
        }, { includeSelf: true });
      }
    },
    close: (ctx) => {
      const clientName = ctx.params.clientName || 'Anonymous';
      ctx.broadcast({
        type: 'leave',
        message: `${clientName} has disconnected.`,
        totalClients: ctx.clients.size
      });
    }
  }
});
