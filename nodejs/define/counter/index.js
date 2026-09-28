import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_root',
  description: 'Runtime background counter root. Returns status over HTTP GET and streams live updates over WebSocket.',
  method: 'ANY',
  websocket: true,
  messageSchema: {
    type: 'object',
    properties: {
      action: { type: 'string', enum: ['get', 'start', 'stop', 'reset'] },
      to: { type: 'integer' },
      intervalMs: { type: 'integer', minimum: 50 }
    },
    required: ['action'],
    additionalProperties: false
  },
  handler: async () => ({
    message: 'Runtime counter status',
    ...counterService.getState()
  }),
  ws: {
    open: (ctx) => {
      // Send initial state upon WebSocket connection
      ctx.send({
        type: 'state',
        ...counterService.getState()
      });

      // Stream live counter updates
      const onTick = (state) => {
        ctx.send({ type: 'tick', ...state });
      };
      const onStart = (state) => {
        ctx.send({ type: 'started', ...state });
      };
      const onStop = (state) => {
        ctx.send({ type: 'stopped', ...state });
      };
      const onReset = (state) => {
        ctx.send({ type: 'reset', ...state });
      };

      counterService.on('tick', onTick);
      counterService.on('start', onStart);
      counterService.on('stop', onStop);
      counterService.on('reset', onReset);

      ctx.ws.once('close', () => {
        counterService.off('tick', onTick);
        counterService.off('start', onStart);
        counterService.off('stop', onStop);
        counterService.off('reset', onReset);
      });
    },
    message: (ctx, data) => {
      switch (data.action) {
        case 'start': {
          const state = counterService.start(data.intervalMs);
          ctx.send({ type: 'started', ...state });
          break;
        }
        case 'stop': {
          const state = counterService.stop();
          ctx.send({ type: 'stopped', ...state });
          break;
        }
        case 'reset': {
          const state = counterService.reset(data.to ?? 0);
          ctx.send({ type: 'reset', ...state });
          break;
        }
        case 'get':
        default: {
          ctx.send({ type: 'state', ...counterService.getState() });
          break;
        }
      }
    }
  }
});
